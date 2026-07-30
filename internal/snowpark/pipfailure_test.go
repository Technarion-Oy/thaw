// SPDX-License-Identifier: GPL-3.0-or-later

package snowpark

import (
	"strings"
	"testing"
)

// The reporter's log from issue #885, trimmed: a requirements file pinning
// pandas==2.0.3 against a venv on Python 3.14. pip resolves the pure-Python
// deps fine, reports no conflict anywhere, then falls back to the sdist because
// no cp314 wheel exists and the setup.py build dies on pkg_resources.
func sourceBuildOutput() []string {
	return strings.Split(strings.TrimSpace(`
Requirement already satisfied: snowflake-snowpark-python==1.54.0 in ./venv/lib/python3.14/site-packages (from -r requirements.txt (line 1)) (1.54.0)
Collecting streamlit==1.41.1 (from -r requirements.txt (line 2))
  Using cached streamlit-1.41.1-py2.py3-none-any.whl.metadata (8.5 kB)
Collecting pandas==2.0.3 (from -r requirements.txt (line 3))
  Using cached pandas-2.0.3.tar.gz (5.3 MB)
  Installing build dependencies: started
  Installing build dependencies: finished with status 'done'
  Getting requirements to build wheel: started
  Getting requirements to build wheel: finished with status 'error'
  error: subprocess-exited-with-error

  x Getting requirements to build wheel did not run successfully.
  | exit code: 1
  +-> [21 lines of output]
      Traceback (most recent call last):
        File "<string>", line 2, in <module>
      ModuleNotFoundError: No module named 'pkg_resources'
      [end of output]

  note: This error originates from a subprocess, and is likely not a problem with pip.
error: subprocess-exited-with-error
ERROR: Failed to build 'pandas' when getting requirements to build wheel
`), "\n")
}

func TestClassifyPipOutput(t *testing.T) {
	cases := []struct {
		name     string
		lines    []string
		wantKind string
		wantSpec string
	}{
		{
			name:     "sdist fallback build failure (issue #885)",
			lines:    sourceBuildOutput(),
			wantKind: pipFailSourceBuild,
			wantSpec: "pandas==2.0.3",
		},
		{
			name: "legacy setup.py wheel build failure",
			lines: []string{
				"Collecting numpy==1.21.0",
				"  Downloading numpy-1.21.0.zip (10.3 MB)",
				"  Building wheel for numpy (setup.py): finished with status 'error'",
				"  ERROR: Failed building wheel for numpy",
				"ERROR: Failed to build numpy",
			},
			wantKind: pipFailSourceBuild,
			wantSpec: "numpy==1.21.0",
		},
		{
			name: "PEP 517 wheel build failure names the project",
			lines: []string{
				"Collecting pyarrow==8.0.0",
				"  Downloading https://files.pythonhosted.org/packages/aa/bb/pyarrow-8.0.0.tar.gz (1.0 MB)",
				"error: subprocess-exited-with-error",
				"ERROR: Failed to build installable wheels for some pyproject.toml based projects (pyarrow)",
			},
			wantKind: pipFailSourceBuild,
			wantSpec: "pyarrow==8.0.0",
		},
		{
			name: "no wheel for this interpreter, other versions exist",
			lines: []string{
				"ERROR: Could not find a version that satisfies the requirement pandas==2.0.3 (from versions: 2.1.1, 2.2.0, 2.2.3)",
				"ERROR: No matching distribution found for pandas==2.0.3",
			},
			wantKind: pipFailWheelMismatch,
			wantSpec: "pandas==2.0.3",
		},
		{
			name: "project absent from the index",
			lines: []string{
				"ERROR: Could not find a version that satisfies the requirement pandsa (from versions: none)",
				"ERROR: No matching distribution found for pandsa",
			},
			wantKind: pipFailNotFound,
			wantSpec: "pandsa",
		},
		{
			name: "bare 'No matching distribution' with no version list",
			lines: []string{
				"ERROR: No matching distribution found for internal-lib==1.0.0",
			},
			wantKind: pipFailNotFound,
			wantSpec: "internal-lib==1.0.0",
		},
		// Pass-through cases: pip's own message is the whole story.
		{
			name: "network failure",
			lines: []string{
				"WARNING: Retrying (Retry(total=4, connect=None, read=None, redirect=None, status=None)) after connection broken by 'NewConnectionError'",
				"ERROR: Could not install packages due to an OSError: HTTPSConnectionPool(host='pypi.org', port=443): Max retries exceeded",
			},
		},
		{
			name: "permission failure",
			lines: []string{
				"ERROR: Could not install packages due to an OSError: [Errno 13] Permission denied: '/usr/lib/python3.12/site-packages'",
			},
		},
		{
			name: "dependency conflict (a real conflict, not a wheel gap)",
			lines: []string{
				"ERROR: Cannot install pandas==2.0.3 and snowflake-snowpark-python 1.54.0 because these package versions have conflicting dependencies.",
				"The conflict is caused by:",
				"    The user requested pandas==2.0.3",
			},
		},
		{
			name: "sdist built fine, unrelated later failure",
			lines: []string{
				"Collecting oldpkg==1.0",
				"  Using cached oldpkg-1.0.tar.gz (12 kB)",
				"  Building wheel for oldpkg (setup.py): finished with status 'done'",
				"ERROR: Could not install packages due to an OSError: [Errno 28] No space left on device",
			},
		},
		{
			name:  "empty output",
			lines: nil,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := classifyPipOutput(c.lines)
			if got.Kind != c.wantKind {
				t.Fatalf("Kind = %q, want %q", got.Kind, c.wantKind)
			}
			if got.Spec != c.wantSpec {
				t.Errorf("Spec = %q, want %q", got.Spec, c.wantSpec)
			}
		})
	}
}

// The stdout "Using cached …tar.gz" line and the stderr "Failed to build" line
// arrive on separate pipes, so classification must not depend on their order.
func TestClassifyPipOutput_StreamOrderIndependent(t *testing.T) {
	lines := sourceBuildOutput()
	reversed := make([]string, 0, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		reversed = append(reversed, lines[i])
	}
	got := classifyPipOutput(reversed)
	if got.Kind != pipFailSourceBuild || got.Spec != "pandas==2.0.3" {
		t.Fatalf("reversed output: got %+v, want source-build pandas==2.0.3", got)
	}
}

func TestPipFailureAdvice(t *testing.T) {
	t.Run("source build names the spec, the interpreter and the fix", func(t *testing.T) {
		msg := describePipFailure(sourceBuildOutput(), "3.14")
		for _, want := range []string{"pandas==2.0.3", "Python 3.14", "not a conflict", "Setup Environment"} {
			if !strings.Contains(msg, want) {
				t.Errorf("advice %q missing %q", msg, want)
			}
		}
	})

	t.Run("unknown interpreter version omits the version", func(t *testing.T) {
		msg := describePipFailure(sourceBuildOutput(), "")
		if strings.Contains(msg, "Python 3") {
			t.Errorf("advice should not claim a version it does not know: %q", msg)
		}
		if !strings.Contains(msg, "the Snowpark environment's Python") {
			t.Errorf("advice %q lost the interpreter phrase", msg)
		}
	})

	t.Run("not-found points at the registry settings", func(t *testing.T) {
		msg := describePipFailure([]string{
			"ERROR: Could not find a version that satisfies the requirement internal-lib (from versions: none)",
		}, "3.12")
		if !strings.Contains(msg, "private registry") {
			t.Errorf("advice %q should mention the private registry", msg)
		}
	})

	t.Run("unrecognized failure yields no advice", func(t *testing.T) {
		if msg := describePipFailure([]string{"ERROR: something else entirely"}, "3.12"); msg != "" {
			t.Errorf("advice = %q, want empty", msg)
		}
	})
}

func TestSpecName(t *testing.T) {
	cases := map[string]string{
		"pandas==2.0.3":  "pandas",
		"pandas>=2.1,<3": "pandas",
		"snowflake-snowpark-python[pandas]==1.54.0": "snowflake-snowpark-python",
		"pandas":      "pandas",
		"pandas~=2.0": "pandas",
	}
	for spec, want := range cases {
		if got := specName(spec); got != want {
			t.Errorf("specName(%q) = %q, want %q", spec, got, want)
		}
	}
}

func TestCondaPythonVersionFallsBackToDefault(t *testing.T) {
	if !isSupportedCondaPython(DefaultCondaPython) {
		t.Fatalf("DefaultCondaPython %q is not in CondaPythonVersions", DefaultCondaPython)
	}
	for _, v := range []string{"", "2.7", "3.7", "3.99", "3.12 && rm -rf /"} {
		if isSupportedCondaPython(v) {
			t.Errorf("isSupportedCondaPython(%q) = true, want false", v)
		}
	}
}
