// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: SQL Editor & Diagnostics

import { useCallback, useEffect, useRef, useState } from "react";
import L from "leaflet";
import "leaflet/dist/leaflet.css";

interface Props {
  /** A parsed GeoJSON object (geometry, Feature, or FeatureCollection). */
  geojson: unknown;
}

/**
 * Renders a single GeoJSON value on a Leaflet map with OpenStreetMap tiles.
 * Lazy-loaded (via React.lazy in CellDetailPanel) so Leaflet only ships when a
 * geo cell is actually inspected. Uses imperative Leaflet rather than
 * react-leaflet — one dependency, no React-version coupling.
 *
 * The map instance lives for the component's whole lifetime: switching between
 * geo cells swaps only the GeoJSON layer, so the user's zoom is preserved
 * (the map recenters on the new geometry but keeps the current zoom). The very
 * first geometry auto-fits. Closing the Map view or running a new query unmounts
 * this component, which destroys the map and resets the view on next open.
 *
 * Points render as circle markers (Leaflet's default marker icon references
 * image files that bundlers don't resolve, so we avoid it entirely).
 */
export default function GeoMapView({ geojson }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const mapRef = useRef<L.Map | null>(null);
  const layerRef = useRef<L.GeoJSON | null>(null);
  const hasFitRef = useRef(false);
  // Shown when tiles fail to load (offline, blocked host, proxy needed). A
  // single successful tile clears it, so a stray edge-of-world error can't
  // leave a false banner up.
  const [tileError, setTileError] = useState(false);

  // Perform the one-time auto-fit for the first geometry. No-ops until the
  // container has a real (non-zero) size, so an early 0×0 measurement can't
  // compute a bogus world view and permanently lock hasFitRef — the fit is
  // retried (from the geojson effect and the ResizeObserver) until it lands on
  // a real size. Invalid/empty bounds fall back to a world view but keep
  // hasFitRef false so a later real geometry still fits properly.
  const fitInitial = useCallback(() => {
    const map = mapRef.current;
    const layer = layerRef.current;
    if (!map || !layer || hasFitRef.current) return;
    const el = containerRef.current;
    if (!el || el.clientWidth === 0 || el.clientHeight === 0) return;

    const bounds = layer.getBounds();
    if (!bounds.isValid()) {
      map.setView([0, 0], 1);
      return;
    }
    // maxZoom caps a single Point (zero-area bounds) from zooming to street level.
    map.fitBounds(bounds, { padding: [20, 20], maxZoom: 14 });
    hasFitRef.current = true;
  }, []);

  // Create the map once; destroy on unmount (Map-view close / new query).
  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;

    const map = L.map(el);
    // Plain-text attribution only: any <a href> in the attribution (Leaflet's
    // default "Leaflet" prefix, or a linked tile attribution) navigates the
    // whole WKWebView with no way back, trapping the user. Drop the prefix link
    // and keep OSM credit as unlinked text.
    map.attributionControl.setPrefix(false);
    L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 19,
      attribution: "© OpenStreetMap contributors",
    })
      .on("tileerror", () => setTileError(true))
      .on("tileload", () => setTileError(false))
      .addTo(map);
    mapRef.current = map;

    // Leaflet measures the container once at init and caches the size, only
    // re-measuring on a *window* resize (its trackResize handler). Any other
    // container-size change — the lazy panel/flex layout settling after mount,
    // classic scrollbars appearing on Windows/WebView2, the cell-detail panel's
    // drag handle, the editor/results splitter — is invisible to Leaflet and
    // permanently corrupts tile ranges, vector-pane positioning, and fitBounds.
    // A ResizeObserver that calls invalidateSize keeps the cached size in sync,
    // and lets us run the deferred first fit once a real size arrives. Same
    // pattern as TerminalPanel's xterm FitAddon and QueryLogPane.
    const ro = new ResizeObserver(() => {
      const m = mapRef.current;
      if (!m) return;
      m.invalidateSize(false);
      if (!hasFitRef.current) fitInitial();
    });
    ro.observe(el);

    return () => {
      ro.disconnect();
      map.remove();
      mapRef.current = null;
      layerRef.current = null;
      hasFitRef.current = false;
    };
  }, [fitInitial]);

  // Swap the GeoJSON layer when the selected cell changes, preserving the
  // current zoom across switches (only the first geometry auto-fits).
  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;

    if (layerRef.current) {
      map.removeLayer(layerRef.current);
      layerRef.current = null;
    }

    try {
      const layer = L.geoJSON(geojson as any, {
        pointToLayer: (_f, latlng) => L.circleMarker(latlng, { radius: 6 }),
      }).addTo(map);
      layerRef.current = layer;

      if (hasFitRef.current) {
        // Already auto-fit once: keep the user's zoom, just move to the new
        // geometry.
        const bounds = layer.getBounds();
        if (bounds.isValid()) map.setView(bounds.getCenter(), map.getZoom());
      } else {
        // First geometry: fit now if the container is measured; otherwise the
        // ResizeObserver runs this once a real size arrives.
        fitInitial();
      }
    } catch {
      if (!hasFitRef.current) map.setView([0, 0], 1);
    }
  }, [geojson, fitInitial]);

  return (
    <div style={{ position: "relative", width: "100%", height: "100%", minHeight: 200 }}>
      <div ref={containerRef} style={{ width: "100%", height: "100%", minHeight: 200 }} />
      {tileError && (
        <div
          style={{
            position: "absolute",
            top: 8,
            left: 8,
            right: 8,
            zIndex: 1000,
            padding: "6px 10px",
            fontSize: 11,
            lineHeight: 1.4,
            color: "var(--text)",
            background: "var(--bg-raised)",
            border: "1px solid var(--border)",
            borderRadius: 4,
            boxShadow: "0 1px 4px rgba(0,0,0,0.25)",
          }}
        >
          Can't load map tiles from <code>tile.openstreetmap.org</code>. This view needs internet
          access to that server. Check your connection, or if you're behind a corporate proxy,
          configure it in your operating system's network settings — the app uses the system proxy
          for map tiles.
        </div>
      )}
    </div>
  );
}
