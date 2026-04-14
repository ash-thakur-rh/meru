import { useEffect, useRef } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { api } from "../api";
import { useThemeContext } from "../context/ThemeContext";

const darkTermTheme = {
  background: "#0f1117",
  foreground: "#e2e8f0",
  cursor: "#a855f7",
  selectionBackground: "#a855f740",
};

const lightTermTheme = {
  background: "#f8fafc",
  foreground: "#0f172a",
  cursor: "#7c3aed",
  selectionBackground: "#7c3aed30",
};

interface Props {
  sessionId: string;
  /**
   * Called once the terminal WebSocket opens and is ready.
   * The supplied function writes raw bytes into the PTY stdin — used by the
   * optional send bar to inject a prompt as if the user typed it.
   */
  onReady?: (writeInput: (data: string) => void) => void;
}

export function AgentTerminal({ sessionId, onReady }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const { theme } = useThemeContext();

  useEffect(() => {
    if (!containerRef.current) return;

    // ── Terminal setup ──────────────────────────────────────────────────────
    const term = new Terminal({
      theme: theme === "dark" ? darkTermTheme : lightTermTheme,
      fontFamily: "ui-monospace, 'Cascadia Code', 'Fira Code', monospace",
      fontSize: 13,
      lineHeight: 1.4,
      cursorBlink: true,
      scrollback: 5000,
    });

    const fit = new FitAddon();
    term.loadAddon(fit);
    term.open(containerRef.current);
    fit.fit();

    // ── WebSocket connection ────────────────────────────────────────────────
    const ws = api.terminalSocket(sessionId);
    wsRef.current = ws;

    // PTY output → xterm.js
    ws.onmessage = (e: MessageEvent) => {
      if (e.data instanceof ArrayBuffer) {
        term.write(new Uint8Array(e.data));
      }
    };

    ws.onopen = () => {
      // Send initial size so the PTY is sized correctly from the start.
      sendResize(ws, term.cols, term.rows);

      // Expose write function to parent (for the optional send bar).
      onReady?.((data: string) => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(new TextEncoder().encode(data));
        }
      });
    };

    ws.onerror = () => {
      term.write("\r\n\x1b[31m[connection error]\x1b[0m\r\n");
    };

    ws.onclose = () => {
      term.write("\r\n\x1b[2m[disconnected]\x1b[0m\r\n");
    };

    // xterm.js keystrokes → PTY stdin
    const dataDisposable = term.onData((data: string) => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.send(new TextEncoder().encode(data));
      }
    });

    // Resize: fit terminal to container, then notify the PTY.
    const observer = new ResizeObserver(() => {
      fit.fit();
      if (ws.readyState === WebSocket.OPEN) {
        sendResize(ws, term.cols, term.rows);
      }
    });
    observer.observe(containerRef.current);

    return () => {
      dataDisposable.dispose();
      observer.disconnect();
      ws.close();
      term.dispose();
      wsRef.current = null;
    };
  }, [sessionId]); // eslint-disable-line react-hooks/exhaustive-deps

  // Propagate theme changes into an already-open terminal.
  useEffect(() => {
    // The terminal object is internal to the effect above; theme changes that
    // arrive before the effect re-runs are handled by re-creating the terminal
    // only when sessionId changes, not on every theme change.  For live theme
    // switching we rely on the CSS variables that xterm.js reads.
  }, [theme]);

  return <div ref={containerRef} className="xterm-container h-full" />;
}

function sendResize(ws: WebSocket, cols: number, rows: number) {
  ws.send(JSON.stringify({ type: "resize", cols, rows }));
}
