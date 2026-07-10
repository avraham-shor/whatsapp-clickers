//go:build pin_deps

// This file is never built; it pins module dependencies that the
// architecture declares but no code imports yet. coder/websocket is first
// imported by the WebSocket hub (Epic 2, Story 2.3).
package deps

import _ "github.com/coder/websocket"
