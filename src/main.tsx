import ReactDOM from "react-dom/client";
import App from "./App";
import { PinnedTerminalWindowApp, pinnedSessionFromLocation } from "./pinned-window";

// R55 · one document, two roles. The main window renders the launcher; the
// independent pinned-terminal window loads the same bundle with
// `?pinned=<brokerSessionId>` and renders only that session's terminal.
const pinnedSession = pinnedSessionFromLocation(window.location.search);

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  pinnedSession ? <PinnedTerminalWindowApp brokerSessionId={pinnedSession} /> : <App />,
);
