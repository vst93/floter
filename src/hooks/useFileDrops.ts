// R7-10a · the file-drop listener and the state a drop owns.
//
// One job: turn a `tauri://drag-drop` payload into the launcher's dropped-file
// state. Nothing here executes anything. The whole module is a subscription
// plus three pieces of state (the files, whether the "…and N more" row has been
// expanded, which of the three actions the switcher is showing); the actions
// themselves are run only from `useLauncherActions` on an explicit Enter/click.
//
// The two guards live here because this is the layer that owns the payload:
//   * `acceptsFileDrop(label, mode)` — only the main window's launcher, never a
//     terminal or settings mode, and
//   * the drop is described by `resolve_dropped_files` and *nothing else*; the
//     handler contains no opener, no terminal and no clipboard call.

import { invoke } from "@tauri-apps/api/core";
import { getCurrentWebview } from "@tauri-apps/api/webview";
import { useCallback, useEffect, useState, type RefObject } from "react";
import {
  acceptsFileDrop,
  nextFileActionIndex,
  type DroppedFile,
} from "../launcher/file-drops";
import type { ViewMode } from "../App";

/** The `tauri://drag-drop` payload as the webview reports it. Only the `drop`
 *  variant carries paths; `enter`/`over`/`leave` are hover traffic and are
 *  deliberately not even read, so hovering a file cannot change any state. */
type DragDropPayload =
  | { type: "enter"; paths: string[] }
  | { type: "over" }
  | { type: "drop"; paths: string[] }
  | { type: "leave" };

export function useFileDrops(options: {
  /** The surface the app is on, read at event time rather than at subscribe
   *  time: the listener is installed once for the app's lifetime. */
  modeRef: RefObject<ViewMode>;
  /** Selection side effects of a drop landing: the first file row, and not the
   *  action bar, is where the keyboard is. */
  setSelectedResultIndex: (index: number) => void;
  setSelectedActionBar: (selected: boolean) => void;
}) {
  const { modeRef, setSelectedResultIndex, setSelectedActionBar } = options;
  const [droppedFiles, setDroppedFiles] = useState<DroppedFile[]>([]);
  const [dropsExpanded, setDropsExpanded] = useState(false);
  const [fileActionIndex, setFileActionIndex] = useState(0);

  useEffect(() => {
    // `onDragDropEvent` subscribes to `tauri://drag-drop` (and its three hover
    // siblings) in the webview; the listener is removed on unmount.
    let disposed = false;
    let unlisten: (() => void) | undefined;

    getCurrentWebview()
      .onDragDropEvent((event) => {
        const payload = event.payload as DragDropPayload;
        // Hover traffic is not a drop. `acceptsFileDrop` then answers the window
        // question: the terminal and settings modes of the main window are not
        // a launcher and must ignore the drag entirely.
        if (payload.type !== "drop") return;
        const paths = payload.paths;
        if (!paths.length) return;
        const label = getCurrentWebview().label;
        if (!acceptsFileDrop(label, modeRef.current)) return;

        // A description, not an action. The backend expands `~`, makes the path
        // absolute, drops what does not exist and re-checks the label; its
        // answer is what the rows render. Nothing is opened, started or copied
        // here — that is the user's Enter on a row (R7-10a red line 1).
        invoke<DroppedFile[]>("resolve_dropped_files", {
          paths,
          windowLabel: label,
        })
          .then((files) => {
            if (disposed || !files.length) return;
            setDroppedFiles(files);
            setDropsExpanded(false);
            setFileActionIndex(0);
            // The drop lands on the launcher's front door: the first file row is
            // the selection, and the action bar is the file's secondary control
            // rather than the thing Enter runs.
            setSelectedActionBar(false);
            setSelectedResultIndex(0);
          })
          .catch(() => undefined);
      })
      .then((dispose) => {
        if (disposed) dispose();
        else unlisten = dispose;
      });

    return () => {
      disposed = true;
      unlisten?.();
    };
  }, [modeRef, setSelectedActionBar, setSelectedResultIndex]);

  /** Drop the dropped rows. Wired into the launcher's existing clear path
   *  (leaving the input surface, and the reveal that follows a hide), so a drop
   *  does not need a dismissal rule of its own. */
  const clearDrops = useCallback(() => {
    setDroppedFiles([]);
    setDropsExpanded(false);
    setFileActionIndex(0);
  }, []);

  /** Run the "…and N more" row: reveal every dropped file. */
  const expandDrops = useCallback(() => setDropsExpanded(true), []);

  /** Move the action switcher one step, wrapping at both ends. */
  const cycleFileAction = useCallback(
    (direction: -1 | 1) => setFileActionIndex((index) => nextFileActionIndex(index, direction)),
    [],
  );

  // A re-drop replaces the file a bar was describing, so the switcher returns to
  // the lead action ("open") rather than carrying a stale selection across.
  const filesKey = droppedFiles.map((file) => file.path).join("\n");
  useEffect(() => {
    setFileActionIndex(0);
  }, [filesKey]);

  return {
    droppedFiles,
    dropsExpanded,
    fileActionIndex,
    clearDrops,
    expandDrops,
    cycleFileAction,
  };
}
