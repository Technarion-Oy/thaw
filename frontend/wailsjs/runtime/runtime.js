// Minimal shim so the app compiles without the full Wails runtime injected.
// The real runtime is injected by Wails at startup via /wails/runtime.js.
export const Call = (method, args = []) => window.go[method](...args);
