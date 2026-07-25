// SPDX-License-Identifier: GPL-3.0-or-later

package ddl

import (
	"sort"
	"strings"
)

// ─── overload naming strategies ──────────────────────────────────────────────

// OverloadNaming selects how overloaded FUNCTION / PROCEDURE definitions — same
// name, different argument signatures — are laid out on disk. A name can carry
// any number of overloads, so a plain <name>.sql layout cannot hold them all;
// the strategies differ in how much of the signature is folded into the file
// name, or whether the overloads share one file.
type OverloadNaming string

const (
	// OverloadNamingArgTypes appends the size-stripped argument type list
	// (historical behavior, and the default):
	//
	//	FOO(X FLOAT, Y VARCHAR(256))  →  FOO__FLOAT_VARCHAR.sql
	//	FOO()                         →  FOO__noargs.sql
	//
	// Overloads that differ only in a size qualifier — VARCHAR(16) vs
	// VARCHAR(256) — produce the same name; planFiles disambiguates them with a
	// numeric suffix.
	OverloadNamingArgTypes OverloadNaming = "argtypes"

	// OverloadNamingSignature keeps the size qualifiers, so size-only overloads
	// land in distinct files without a numeric suffix:
	//
	//	FOO(X VARCHAR(16))   →  FOO__VARCHAR_16.sql
	//	FOO(X NUMBER(38,0))  →  FOO__NUMBER_38_0.sql
	OverloadNamingSignature OverloadNaming = "signature"

	// OverloadNamingGrouped drops the suffix entirely: every overload of one
	// name goes into a single FOO.sql, one CREATE statement per overload in
	// signature order.
	OverloadNamingGrouped OverloadNaming = "grouped"
)

// DefaultOverloadNaming is the strategy used for the zero value, preserving the
// layout produced before the strategy was configurable.
const DefaultOverloadNaming = OverloadNamingArgTypes

// normalize maps the zero value — and any unknown string arriving from the
// frontend or a hand-edited config file — onto the default, so no caller has to
// validate the value first.
func (n OverloadNaming) normalize() OverloadNaming {
	switch n {
	case OverloadNamingSignature, OverloadNamingGrouped:
		return n
	default:
		return DefaultOverloadNaming
	}
}

// overloadSuffix returns the "__<signature>" file-name suffix for an
// overloadable object. It is empty when the object is not a FUNCTION /
// PROCEDURE, when the strategy groups overloads into one file, or when Parse
// found no argument list to derive a signature from.
func (o *Object) overloadSuffix(naming OverloadNaming) string {
	if o.Kind != KindFunction && o.Kind != KindProcedure {
		return ""
	}
	switch naming.normalize() {
	case OverloadNamingGrouped:
		return ""
	case OverloadNamingSignature:
		if o.ArgSigFull != "" {
			return "__" + o.ArgSigFull
		}
		// No full signature parsed (older callers that only set ArgSig, or an
		// unreadable argument list) — fall back to the size-stripped form.
	}
	if o.ArgSig == "" {
		return ""
	}
	return "__" + o.ArgSig
}

// ─── file planning ───────────────────────────────────────────────────────────

// filePlan pairs one output path with the object(s) whose DDL is written there.
// Objects holds more than one element only when overloads are grouped.
type filePlan struct {
	Path    string
	Objects []Object
}

// content renders the file body: every statement terminated with a semicolon,
// separated by a blank line when several overloads share the file.
func (p filePlan) content() []byte {
	var b strings.Builder
	for i, o := range p.Objects {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(o.SQL)
		b.WriteString(";\n")
	}
	return []byte(b.String())
}

// planFiles maps parsed objects onto relative output paths.
//
// The result depends only on the *set* of objects, never on the order
// Snowflake happened to return them in, so filenames stay stable across
// exports and diffs in a git working tree remain meaningful:
//
//  1. Objects are grouped by candidate path, and the groups are ordered by
//     path.
//  2. Every candidate path is reserved up front, so a group forced onto a
//     numeric suffix skips names a real object already owns.
//  3. Within a group, members are ordered by their signature (never by
//     position in the DDL stream): the first keeps the plain path and the rest
//     take the next free _2, _3, … slot.
//
// Under OverloadNamingGrouped a group that consists purely of overloads of one
// name is not split at all — the members share a single file in that same
// signature order. Collisions between *unrelated* objects (a template without
// {schema} flattening two schemas, say) still get numeric suffixes.
func planFiles(objs []Object, template, database string, naming OverloadNaming) []filePlan {
	naming = naming.normalize()

	type group struct {
		path    string
		members []Object
	}
	groups := make([]*group, 0, len(objs))
	byPath := make(map[string]*group, len(objs))
	for _, o := range objs {
		p := o.FilePathFor(template, database, naming)
		g := byPath[p]
		if g == nil {
			g = &group{path: p}
			byPath[p] = g
			groups = append(groups, g)
		}
		g.members = append(g.members, o)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].path < groups[j].path })

	tracker := newNameTracker()
	for _, g := range groups {
		tracker.resolve(g.path) // distinct by construction — reserves, never renames
	}

	plans := make([]filePlan, 0, len(objs))
	for _, g := range groups {
		if len(g.members) == 1 {
			plans = append(plans, filePlan{Path: g.path, Objects: g.members})
			continue
		}
		sort.SliceStable(g.members, func(i, j int) bool {
			return overloadKey(g.members[i]) < overloadKey(g.members[j])
		})
		if naming == OverloadNamingGrouped && areOverloadsOfOneName(g.members) {
			plans = append(plans, filePlan{Path: g.path, Objects: g.members})
			continue
		}
		for i := range g.members {
			path := g.path
			if i > 0 {
				path = tracker.resolve(g.path)
			}
			plans = append(plans, filePlan{Path: path, Objects: g.members[i : i+1]})
		}
	}
	return plans
}

// overloadKey is the stable sort key ordering the members of a colliding group.
// It is derived from the object itself — never from its position in the DDL
// stream — so both numeric-suffix assignment and grouped-file order reproduce
// exactly on the next export.
func overloadKey(o Object) string {
	return o.ArgSigFull + "\x00" + o.ArgSig + "\x00" + o.SQL
}

// areOverloadsOfOneName reports whether every member is an overload of the same
// function or procedure — the only case where sharing a single file is
// meaningful.
func areOverloadsOfOneName(members []Object) bool {
	first := members[0]
	if first.Kind != KindFunction && first.Kind != KindProcedure {
		return false
	}
	for _, m := range members[1:] {
		if m.Kind != first.Kind ||
			!strings.EqualFold(m.Name, first.Name) ||
			!strings.EqualFold(m.Schema, first.Schema) ||
			!strings.EqualFold(m.Database, first.Database) {
			return false
		}
	}
	return true
}
