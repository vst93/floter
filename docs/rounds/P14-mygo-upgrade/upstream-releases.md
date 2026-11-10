# v0.8.3

https://github.com/egoist/mygo/releases/tag/v0.8.3

Published: 2026-11-07T08:56:09Z

## Changes in behavior

No breaking changes to the Go API. Two changes code may notice:

- **Markdown's default parser reads GitHub mentions and references.** `parser.Default` now includes alerts, mentions and issue references, so code that walks a document's inlines gets `Mention` and `Reference` inlines where it used to get `Text`. Their `Text` field holds the text as written, and `parser.PlainText` and the HTML writer handle them. To keep the old output, parse with `parser.Default &^ (parser.ExtMentions | parser.ExtReferences)`. (#1423)
- **Generated TypeScript types named `Record` or `Promise` get their package's prefix,** such as `ModelsRecord`, so they no longer hide TypeScript's built-in `Record` and `Promise` and the client compiles. Code importing those types uses the new names after regenerating. JSON fields and method names are unchanged. (#1421, @HiPanJin)

## Highlights

- **GitHub alerts, mentions and issue references in Markdown.** `> [!NOTE]`, `[!TIP]`, `[!IMPORTANT]`, `[!WARNING]` and `[!CAUTION]` render as GitHub's alerts with their icons and colors, also while a model's answer is still streaming in, so a chat app never shows the marker as text. `@octocat` and `#123`, `GH-123` or `octocat/hello#123` are read as GitHub reads them. With `Options.Resolve`, an issue or PR viewer turns them into links that open the person or the PR in place, and `Options.Preview` shows a hover card on any link, mention or reference. `Theme.Alerts` holds the alert colors. (#1423, @brodienguyen)
- **Korean text breaks lines between words.** `Text`, `TextInput` and `TextArea` keep Korean words whole and break at spaces, as CSS's `word-break: keep-all` does, whenever the text's language is Korean (from the app's language or a span's `Lang`). A word longer than the line still breaks. (#1405, requested by @yldst-dev in #1403, who wrote the patch it builds on)

## New

- `Page.GPUProcessID()` gives the process that does a page's graphics and media, so a music player can capture a web player's sound for a visualizer with a Core Audio process tap on macOS. On Windows it is WebView2's GPU process, and on Linux it is 0. (#1401, requested by @fzn0x in #1397)

## Fixes

### UI and apps
- Space in an element that takes text itself (`HandleTextInput` or `TextCaret`) inside a scroll view types a space without paging the view. (#1411, reported by @tacshi in #1410)
- In a split diff, a hunk's header stays on the old side and ends in an ellipsis before the divider, however far the divider is dragged. (#1415, reported by @yangdev02 in #1413)
- `Tester.RightClick` on a list row opens the context menu that the row's content carries. (#1416, reported by @yangdev02 in #1414)

### Graphics
- A clip inside a rounded, clipped view, such as a column filling the bottom of a rounded card, keeps to the card's rounded corners with Metal, Direct3D and OpenGL. (#1420, reported by @Mrhoho in #1418)
- A card turned over in perspective far from the window's top left stays visible, and clips inside it keep to its size. (#1409, reported by @fzn0x in #1406)
- A rounded view that grows over an animation, such as a card opening out of a cover until it fills the window, keeps one mask instead of allocating a new one each frame, so opening it no longer adds a couple of hundred MB of memory. (#1428)
- Idle clip masks keep within their 32 MB memory budget. (#1408, reported by @fzn0x in #1407)

### macOS
- With the Korean 2-Set input source, pressing Return while a syllable is composing commits it and also reaches the app, so a chat box sends on the first press. (#1404, reported by @yldst-dev in #1402)

### Windows
- Animations that change only every other frame free their idle textures. (#1394)

### Linux
- `Window.SetIgnoreMouseEvents(true)` lets clicks through to what is under the window, as on macOS and Windows, for overlays such as desktop lyrics. (#1419, @r0k1s-i)

### Updates
- Apps built with this version or later remove the files that a newer version no longer ships when they update on Linux and Windows. (#1400, reported by @fzn0x in #1396)

## New Contributors

- @r0k1s-i made their first contribution in #1419
- @brodienguyen made their first contribution in #1423

**Full Changelog**: https://github.com/egoist/mygo/compare/v0.8.1...v0.8.3


# v0.8.1

https://github.com/egoist/mygo/releases/tag/v0.8.1

Published: 2026-11-06T18:37:01Z

## Highlights

- **macOS: animated apps no longer pile up GPU memory.** Textures of transient canvases and idle images are freed while an animation runs, so an app with moving lyrics stays small instead of growing past 1.5 GB. (#1393)

## New

- **Menu buttons place and open their menus as the app asks.** `ui.PopoverMenuButtonOptions` lines a drawn menu up with the end of its button, or puts it above or beside the button, and the menu still flips at the window's edges. Its `Open` field lets the app open or close the menu itself, for example when the item the menu acts on goes away in the background. (#1392, requested by @xiyaowong in #1386)
- **Richer items in drawn menus.** `MenuItem.Danger` shows a destructive item such as "Delete…" in the theme's caution color, `Subtitle` adds a muted second line, `Trailing` puts a value such as a file size at the item's end, and `Content` lets an item show anything, such as a download row with a progress bar. `Highlighted` tells the menu which item the pointer or keyboard is on, so it can preview it, `ID` keeps a choice and the highlight on their item as labels change or items come and go, and `Menu.MaxWidth` caps the menu's width with labels ending in an ellipsis. Screen readers read the subtitle and trailing value as the item's description. (#1391, requested by @xiyaowong in #1385)
- **SVG icons and per-item icon colors in menus.** `mygo.NewMenuIconSVG` makes a menu icon from the same `ui.SVG` an app already uses for `ui.Icon`, and `MenuItem.IconColor` tints a template icon in the menus MyGo draws, following the highlight and dimming with a disabled item. `(*ui.SVG).Image` renders an SVG to pixels. (#1392, requested by @xiyaowong in #1387)
- **`ListState.HoldPlace` keeps a chat or log list where it is.** One call in a click handler keeps the clicked line in place when a fold in the last row opens, and the list follows the end again once it shows it. (#1390, requested by @lixw1994 in #1389)

## Fixes

### UI and apps
- A `List` reads `FollowEnd` at each layout, so turning it off or on takes effect in the same frame. (#1390, reported by @lixw1994 in #1389)

### TypeScript bindings
- A bound Go method with a parameter named `call` generates TypeScript that type-checks. (#1388, @HiPanJin)

**Full Changelog**: https://github.com/egoist/mygo/compare/v0.8.0...v0.8.1


# v0.8.0

https://github.com/egoist/mygo/releases/tag/v0.8.0

Published: 2026-11-06T17:15:01Z

## Breaking changes

- **Every key reaches `HandleInput`, named by its position.** `InputEvent.Code` is the key by its place on any keyboard, named as the web's `KeyboardEvent.code`: it tells left from right (`ui.CodeShiftLeft`, `ui.CodeAltRight`) and names keys `Key` has no name for, such as Caps Lock, the keypad, Print Screen and the 102nd key. `HandleInput` and `ObserveInput` now get every key the system reports, as a web page's listeners do: modifiers pressed and released alone (a lone tap of Super among them), the lock keys, and keys the layout types no `Key` for arrive as `InputKeyDown` and `InputKeyUp` with `Key` set to `ui.KeyUnknown`. A game or an emulator can now map keys by position, and an app can react to a tap of a modifier alone. (#1328)
  - Handlers that pick keys with a `switch` see no change.
  - Handlers that react to any key, such as "stop the demo on the first key", now also fire on Shift. Check `ev.Key != ui.KeyUnknown`.
  - Handlers that return true for every key now take Alt, so it no longer opens the menu bar on Windows and Linux. Return false for the keys you don't use.
  - `plugins/terminal`'s `OnKey` no longer sees keys without a `Key`, which on macOS and in browsers includes keys such as F13.

## Changes in behavior

- **Inactive windows look inactive.** While another window is in front, a MyGo window fades its accent to gray, as native apps do. To keep the accent in every window, set `Theme.InactiveAccent = Theme.Accent`. (#1022, requested by @xc2 in #1019)
- **`Opacity` fades an element and its children as one, as CSS does.** Overlapping children of a faded element no longer darken each other, so a faded card with a badge over its edge looks the same as on the web. (#1321, requested by @cunninghamcard-bit in #1289)
- **Presses and drags follow the primary button.** `Pressed`, `Dragged`, `DragVelocity` and `DragEnded` no longer start on a right or middle press, so a right-button drag no longer moves a card, a slider, a split or a table column. It still right-clicks and opens context menus. `PressedButton` tells which button presses an element. (#1156)
- **A cancelled press commits nothing.** When the system takes a press, as a window losing the mouse capture or a touch turning into scrolling in a browser, there is no click, drop, `DragEnded` or context menu, and a `Draggable` goes back to where it was picked up. `HandleInput` still gets the release, with `InputEvent.Canceled` set, and `PressCanceled` lets a view roll back a preview. (#1365, requested by @cunninghamcard-bit in #1362)
- **A toast's time keeps running in the background.** A tray app whose main window is hidden no longer shows stale toasts when the user opens the window. A toast whose time ran out while the window was in the background closes without showing. (#1064, #1369, requested by @xiyaowong in #1051)
- **Frames that only animate wait while a window is minimized on Linux and Windows**, as they already did on macOS and the web. (#1325)
- **A `Transition` that names no channel fades shadows too**, as it fades colors. (#1211)
- **`ui.Shape` gives glyphs in visual order on Windows**, as it does on macOS and Linux. (#1165, reported by @ikexing-cn in #1161)
- **`mygo build` no longer ships Go sources from `resources/`**, and `resourcesExclude` in `mygo.json` leaves out other paths there. Dotfiles inside a resource directory now ship, so a package that reads its own `.manifest.json` works once built; hidden directories and `.DS_Store`, `Thumbs.db` and `desktop.ini` stay out. (#1063, requested by @xiyaowong in #1052; #1167, @baka-world, reported in #1166)

## Highlights

- **Edit styles live in the inspector.** As in Chrome's DevTools, a click on a value in the Styles tab edits it, with the window following as you type. A checkbox turns a declaration off, + adds one, and Up and Down step a number. **Copy as Go** writes what you changed as calls to paste into the view. The Styles tab can also force `:hover`, `:active` and the focus states on an element, the flex and grid badges in the tree draw a container's layout over the content, and the `main.go:42` beside an element opens that line in your editor (`MYGO_EDITOR`, `VISUAL` or `EDITOR`). Production builds leave all of it out. (#1146, #1147, #1148, #1160, #1162)
- **`mygo-automation`, a command line for AI agents.** An agent driving the app it develops gets compact text snapshots of a window with short refs (`@e2 [button] "Add"`), acts on them with `press`, `fill` and app keys, and with `--settle` sees a diff of what changed. `mygo-automation skills` prints a SKILL.md that matches the installed version. (#1117, #1175, requested by @fondoger in #1112)
- **Magic Move for shared elements.** `ElementTransition.Fly` flies an element from the one it takes over from, as Keynote's Magic Move and Flutter's Hero do. A playlist tile can open into its page with the cover flying from the grid into the header, its corners, opacity, colors and 3D transforms blending on the way. `ContentTransition.Morph` turns one text into another character by character, so a tile's "Ocean" grows into the page's "Ocean at dusk". `Router.Transition = TransitionMatch` fades pages into each other while their shared elements fly. The gallery's Motion page has a demo. (#890, #1016, @hronro)
- **Faster native UI on Windows.** GPU time per frame is about halved, a scene of 9,600 boxes runs at 60 fps instead of 30, and input under load reaches the screen in 25 ms instead of 43. Pages of text use less memory on every platform, because glyph atlases are sized by the glyphs they hold. (#1012)
- **Faster text on macOS.** Resizing a window of text, sliding a sidebar or dragging a split takes about half the CPU it did: 4.2 ms a frame instead of 8.1 on a document of 40 paragraphs. A dashboard of 600 changing numbers takes 5.0 ms instead of 7.2. (#1318, #1333)
- **Windows 11 menus.** Tray menus open above the taskbar, centered on the icon, as the system's do. Menus MyGo draws get Windows 11's Acrylic material, rounded corners and shortcut labels, with a solid fallback when transparency effects are off or a contrast theme is on. (#1178, requested by @godcop in #1169)
- **Liquid Glass over web views.** On macOS 26, `glass.Glass{Native: true}` puts the system's real Liquid Glass under a native toolbar or sidebar that sits over a page. `glass.Group` merges nearby panes into one surface that bridges and separates as they move, and glass follows Reduce Transparency and Increase Contrast. (#1107, @kyya)
- **Screen readers read hand-built widgets as the web does.** A data grid, a tree, a suggestion list or a form of your own can now say everything ARIA can: grid and tree grid roles with row and column counts, list boxes and options, "3 of 50" and selection, live regions and alerts that the engine announces itself, descriptions, error messages, invalid fields, current items, busy content, keyboard shortcuts, autocomplete, and controls that are unavailable but still focusable. They reach VoiceOver, Narrator and Orca. `Tester.Access` reads an element's role, name and states in tests. (#1104, #1158, #1179, #1199, #1208, #1209, #1256, #1280, #1316, #1319, #1322, #1366, #1371, requested by @cunninghamcard-bit in #1093, #1172, #1181, #1193, #1293, #1295 and #1309)
- **Ask for permissions, and guide users through System Settings.** The new `plugins/permissionflow` checks and requests Accessibility, Screen Recording, Input Monitoring, Full Disk Access, the microphone, the camera and more on macOS. Where the user has to drag the app into a list in System Settings, `Guide` opens the right page and shows a panel with the app's card to drag. On Windows it reads the microphone and camera switches and opens their settings. (#655)
- **Snippets in the code editor.** `editor.ParseSnippet` and `State.InsertSnippet` insert templates such as `print(${1:value})$0`, with fields that are edited together and Tab stops between them, and a `Completion` can carry one. (#1361, @zzstar101)

## New

### Layout and painting
- `Element.Sticky` keeps an element in view inside its scroll container, as CSS's `position: sticky`: a timeline's day heading stays at the top until the next day pushes it out. (#1195)
- `ScrollSnap` and `SnapAlign` snap a scroll container to the places its elements mark, as CSS's scroll snap, for carousels and paged galleries. (#1187)
- `Element.ZIndex` orders an element's painting among its siblings, as CSS's `z-index`. (#1226, requested by @cunninghamcard-bit in #1205)
- `ClipInset`, `ClipInsetPercent`, `Painter.ClipPath`, and `MaskLinearGradient`, `MaskRadialGradient` and `MaskConicGradient` clip and mask an element. `Masks` layers several masks, as CSS's `mask-composite`, and `ObjectPosition` places an image's picture where `Fit` crops it. `ui.ParsePath` reads SVG path data. (#1110, #1210, requested by @cunninghamcard-bit in #1100)
- `Path.FillRule(ui.EvenOdd)` fills and clips a ring with its hole, `Path.Contains` tells whether a point is inside a path, and `Element.HitTest` makes an element take the pointer only where you say, such as inside a round button. (#1315)
- `TranslationZ`, `Translation3D` and `Element.TranslateZ` move content toward or away from the viewer, for depth stacks, carousels and cards that lift as they turn. (#1364, requested by @cunninghamcard-bit in #1363)
- `Element.TextShadow` and `Span.Shadow` cast a shadow of text, so a caption over a photo stands out. `TextStroke` outlines text, and `TabSize` sets tab stops. (#1217, #1327)
- `Element.Balance` wraps a heading into lines of about the same width, and `Pretty` keeps a paragraph's last line from holding a single word. (#1110, #1327)
- `PreserveSpaces` keeps a text's spaces, as `white-space: break-spaces`. (#1201, @cunninghamcard-bit, requested in #1200)
- `CaretColor` sets the color of a text input's caret. (#1215, @cunninghamcard-bit, requested in #1214)
- `NoShadow` takes off the shadows an element has, so a card a helper returns with a shadow can be made flat. (#1267, @cunninghamcard-bit, requested in #1265)
- A `Span` can reset letter spacing with `NoLetterSpacing` and pad its background with `PaddingX` and `PaddingY`, for inline tags and code. (#1257, requested by @cunninghamcard-bit in #1227)
- `Track.Min` and `Track.MinContent` give grid tracks a minimum, as CSS's `minmax(240px, 1fr)`. (#1255, requested by @cunninghamcard-bit in #1240)
- `MaxWidth` and `MaxWidthPercent` apply together, as CSS's `max-width: min(78%, 34rem)` for a chat bubble, and so do the other minimum and maximum sizes. `DimDisabled(false)` keeps a disabled element at full opacity, for a design that styles its own disabled look. (#1176)

### Motion
- `Motion.Blur` blurs an Enter or Exit, `TranslatePercent` and the `XPercent`/`YPercent` of motions and poses move by a share of the element's own size, and `Animation.Oklab` and `Color.MixOklab` mix colors in Oklab. `ElementTransition.AfterExits` holds an element back until its siblings have left, and `Element.ChildExit` lets a page that turns around mid-step leave the new way. (#1106, requested by @cunninghamcard-bit in #1095)
- `ElementTransition` animates shadows (`Shadows`), times exits with `ExitDuration` and `ExitEase`, and animates `Width` or `Height` alone, so a panel springs open while its width follows a drag. `AttachedOrigin` and a motion's `Flip` make a popover grow from the side `AttachTo` put it on. (#1211, #1259, requested by @cunninghamcard-bit in #1253)
- `NonMotion` keeps a fade or a blur animating while the desktop asks for less motion. (#1312, requested by @cunninghamcard-bit in #1290)
- `ScrollIntoView` lines an element up at the start, center or end, and can glide there. (#1273, requested by @cunninghamcard-bit in #1264)

### Input and focus
- `Element.ObserveInput` lets a container see the keys, text and presses inside it before the focused element gets them, as the web's capture phase. (#1221, requested by @cunninghamcard-bit in #1206)
- `Context.KeyDown` and `KeysDown` tell the keys held in the window, whatever has the focus, for a shortcut cheat sheet that lights the keys held. (#1177)
- `Context.ShortcutRepeated` tells a shortcut's auto-repeat from a press, and `Context.OnWindowShortcut` declares a window shortcut from anywhere in the tree. (#1198)
- `Element.FocusLeft` tells the focus leaving an element, so a combobox closes on blur but not as the focus moves into its list. `Context.FocusTakesText` tells whether the focused element takes text. (#1202)
- `TabStop(false)` makes an element focusable but leaves it out of Tab's order, as `tabindex="-1"`. (#1314, requested by @cunninghamcard-bit in #1308)
- `LabelledBy` ties a caption the view draws to its control: it names the control, and a click on it focuses the control or toggles a check box. (#1319, requested by @cunninghamcard-bit in #1309)
- `PressedInside` tells a press inside an element without taking the pointer, so a table cell can notice an Alt-click while its row still chooses. (#431, @saanuregh)
- `DragMissed` tells a drag's source it was let go over no target, and where, for tearing a tab out into a window of its own. (#1351, @saanuregh)
- Presses carry their pointer type and pressure, for pen and touch. More cursors (progress, wait, help, cell, zoom and others), and the result of a clipboard write. (#1204)
- `Preferences.Pointer`, `AnyPointer`, `NoHover` and `NoAnyHover` tell whether the pointer is coarse and can hover, as CSS's media queries, so a touch-first layout can give larger targets. (#1326)
- `Context.WindowHidden` tells whether the window is hidden, minimized or covered, so a dashboard can pause its polling. (#1325)
- `InViewMargin` and `InViewAmount` tell an element in view with a margin, or by a share of its area, as Intersection Observer does, to load a thumbnail ahead of time or count an impression once half a card shows. (#1274, #1324, requested by @cunninghamcard-bit in #1266)
- `MaxLength` limits how long a text input's value can be typed. (#1269, @cunninghamcard-bit, requested in #1261)
- `SplitReset` makes a double click on a split's divider put the pane back at a size you choose. (#1313, @BABA983)
- A value made with `Local` that is an `io.Closer` is closed when its element goes. (#1233, requested by @cunninghamcard-bit in #1225)

### Widgets
- `MenuBase`, `ContextMenuBase` and `AccordionBase` give menus and accordions with your own layout the keyboard and focus of the built-in ones, and `CheckboxStateBase` has a mixed state for a "select all" box. A base's focus ring can be set. (#1108, requested by @cunninghamcard-bit in #1096)
- `TreeBase` is a tree without a look, with a tree's keys and accessibility, and an outline's rows can start with your own content (`Prefix`). (#1155)
- A `MenuBase` item can keep its menu open as it is chosen (`KeepOpen`), and `Group` puts a row of items, such as Light, Dark and System, in one stop of the keys. (#1154)
- A `SelectBase` can be opened and closed by its caller (`SetOpen`). (#1183)
- A `ComboboxBase` can show a "No matches" row. (#1150)
- Menu items take an icon (`Icon`), a subtitle (`Subtitle`) and a tooltip (`Tooltip`). (#1297, #1299, @baranwang, requested in #1296 and #1298; #1367, @saanuregh)
- `Context.SetMenuPresentation(ui.PopoverMenus)` draws every menu of a window in the window, the table's, the code editor's and text inputs' editing menu included. (#1359, @saanuregh)
- `ListState.HeaderCell` builds into a table's header cells, and `ListState.CellsStyle` paints a spreadsheet's chosen cell, its row and stripes. (#432, @saanuregh; #1379, @Kuangcp)
- `ListState.DragRows` drags a list's rows out to targets elsewhere, such as files into a folder row. (#1089, @MichaelC001)
- `AttachEdgeMargin` keeps an attached panel a chosen distance from the window's edges. (#1285, @cunninghamcard-bit, requested in #1283)
- `Theme.TooltipDelay` sets how long the pointer rests before a tooltip shows, for all of a window or part of it. (#1062, requested by @xiyaowong in #1056)
- `Theme.WindowButtons` tells which window controls the desktop's title bars have, so an app drawing its own shows none under Hyprland or Sway. (#1320)
- `KeepScrollbars(true)` keeps a scroll container's or a table's bars showing while its content overflows. (#999, requested by @Binaryify in #998)

### Code editor, terminal and Markdown
- The line between a split diff's sides can be dragged, or moved with the arrow keys, and `DiffState.SplitPercent` holds the share for the app to save. (#1021, requested by @yangdev02 in #1020)
- `DiffState.ScrollY`, `SetScrollY`, `ViewportHeight` and `LineTop` let an app draw a minimap beside a diff. The code editor and the diff view take `KeepScrollbars` and `NoScrollbars`. (#999, requested by @Binaryify in #998)
- `State.HoverDelay` sets when the code editor's hover shows, and `NoPointerHover` shows it only from the keys. (#433, @saanuregh)
- `terminal.Options.ShellIntegration` marks zsh prompts, so zsh redraws its prompt cleanly as the window resizes. (#1077, @daodao97)
- Option-click (Alt-click elsewhere) on the command at a shell's prompt moves the cursor there, as in Ghostty and kitty. (#1085, requested by @Th1nkK1D in #1070)
- `Font.CellWidth` widens the terminal's columns while glyphs keep their size. (#1229, @iFwu)
- The terminal links paths in Chinese and Japanese prose, quoted paths with spaces, and places such as `app.ts(12,3)` and `app.go#L12C3`. (#1302, @daodao97)
- `markdown.Options.LinkMenu` gives each link a context menu of its own, such as Copy Path and Reveal in Finder for a file link, and `Options.LinkIcon` puts an icon before a link's text. (#1136, requested by @miniers in #1130; #1131, @HarriesCC)
- `markdown.Revealed` tells when a streamed reply has finished showing, so a chat can show its time and actions then. (#1381, requested by @tsugumi42 in #1380)

### Platform
- `plugins/sfsymbols` draws Apple's SF Symbols in native UI on macOS, and reports none elsewhere so apps fall back to their own icons. (#1149, @kyya)
- `NotificationOptions.Image` shows a picture with a notification, such as a track's cover. (#1011, requested by @fzn0x in #1009)
- `Shell.TypeIcon` gives the system's icon for a file type with no file on disk. (#984, @baranwang, requested in #983)
- `FileFilter.MIMETypes` filters a file dialog by MIME type, such as `image/*`. (#1325)
- Flyouts of native UI size to their content with `FitContent`, and a window with a flyout as its `Parent` and `Modal` opens over it. (#1069, #1067, requested by @xiyaowong in #1054 and #1055)
- A rounded web view is cut to its corners, and a scaled one shows its page scaled, on every platform. (#1034, requested by @ourongxing in #1026)
- A web view gives the native UI around it the keys its page doesn't handle. (#1040, requested by @ichts in #1036)
- `audio.Player.SetPreservesPitch` keeps a voice's pitch at 1.5x or 2x, for podcast and lecture players. (#1254, requested by @cunninghamcard-bit in #1228)
- On Linux, `Theme.SetGTKCSS` styles GTK's own widgets, such as header bars and title buttons, and `App.SetDMABUFRenderer` turns WebKitGTK's faster DMA-BUF renderer on, everywhere or only on hosts where it's safe. (#1126, #1124, requested by @baka-world in #1121 and #1122)

### Formatting
- `NumberOptions.Notation` writes compact numbers ("1.2K"), `NumberOptions.Currency` amounts of money, and `NumberOptions.SignDisplay` a plus, as in "+3.4%". `FormatDateRange` writes ranges of dates, and `FormatRelativeTime` writes "3 minutes ago" in each language. (#1218, #1258; #1287, @cunninghamcard-bit, requested in #1286; #1348, @zzstar101)

### Build and distribution
- `mygo.json` takes a `license`, used by the PKGBUILD. (#1119, @baka-world, requested in #1118)
- The Linux desktop entry names the window class, so X11 taskbars group the window with its launcher and can pin it. (#1129, @baka-world, reported in #1128)

## Fixes

### UI and apps
- A drag released between two frames reaches the release point, so dividers, columns, sliders, text selection and rows chosen by dragging no longer stop short. (#1008, #1015, @liby)
- A click is no longer swallowed after a press that moved its element. (#1003, reported by @lixw1994 in #1001)
- Apps that keep many fonts in use no longer crash on macOS and Linux when text laid out earlier is drawn again. (#990, reported by @caelyreth in #982)
- Text moved by a fraction of a pixel inside rounded clips no longer costs tens of MB of memory, and text that changes width is laid out faster. (#1005, reported by @saminnet in #1002; #1013, @hronro)
- Long stroked paths, such as a chart's lines, no longer vanish from frames. (#1090, #1113, reported by @SakuraToErii in #1086)
- A closed window lets go of everything it showed, so a screen capture tool no longer keeps old captures in memory. `ReleasePixels` frees a bitmap's pixels at once. (#1282, reported by @douglarek in #1281)
- Code that keeps the window from `NewWindow` in a variable and reads it in the view no longer crashes on the first frame. (#1068, reported by @xiyaowong in #1053)
- `Eval` returns an error when its window closes or the app quits, instead of waiting forever. (#1065)
- The pointer goes through a tooltip to what is under it, so the wheel scrolls the view under a tip. (#1091, @wibus-wee)
- A tooltip shows when the keys move to a menu item or an option, not only under the pointer. (#1368, @saanuregh)
- Popovers animate their content, and their Enter and Exit transitions play, including an Enter's travel on a panel placed with `AttachTo`. (#1137; #1140, #1192, @cunninghamcard-bit, reported in #1139 and #1191)
- A trigger no longer goes dark for a frame when the popup over it closes. (#1377)
- An element focused as it is first built, such as a popover's search field, shows its focus within at once. (#1142, @cunninghamcard-bit, reported in #1141)
- Typing while a press is held no longer moves the caret back to the pointer. (#1159, @1bit-JS, reported in #1152)
- `SelectBase` and `TabsBase` pass over disabled options and tabs, Tab enters a radio group or tab list at the one chosen, and Up opens a menu button on its last item. (#1135, #1153, #1186, @cunninghamcard-bit, reported in #1151 and #1185; #1150)
- A focused link takes Enter alone; Space scrolls, as on the web. (#1278, reported by @cunninghamcard-bit in #1276)
- The calendar's next and previous month buttons no longer skip a month from the 31st. (#1157, @wiptgdxv)
- Turning the wheel back at the end of content turns at once. (#1356, @sugarbearr)
- A `NumberInput` or `DecimalInput` keeps its buttons within the width it is given, and an `AlertDialog` fits or wraps its buttons. (#1349, @saanuregh; #1329)
- A text's `ContentTransition` no longer stays half faded beside a blinking caret. (#1350, @saanuregh)
- A blur or effect inside a `Blur` or `Cache` element is no longer cut off at the element's box. (#1375)
- An inline `Text` fades with its `Opacity`, a transparent color or `Invisible`. (#1272, reported by @cunninghamcard-bit in #1262)
- A text ending in a no-break space keeps it, and an invisible child gives its box a baseline, as on the web. (#1134, #1133, @cunninghamcard-bit)
- Screen readers reach the buttons inside headings and column headers, leave text hidden with `AccessHidden` out of a paragraph's name, read a range's value text, and VoiceOver no longer says "collapsed" on elements that never open. (#1310, @HiPanJin, reported in #1307; #1279, reported by @cunninghamcard-bit in #1275; #1268, #1223, @cunninghamcard-bit, reported in #1260 and #1222; #1188)
- Window states, recent documents and recovery copies no longer read as missing on Windows while they are being saved. (#1138)
- Documents no longer lose edits when a save and an edit cross, and recovery keeps copies it can't restore yet. (#1125, @puresky271)
- A document follows its file when another app renames it. (#1127)
- `Tester`'s clicks on a named element fail when the element is hidden or covered, instead of clicking what is there. (#1116, @echoVic)
- `mygo migrate-ui -write` no longer turns nil checks of values that aren't elements into code that doesn't compile. (#996, reported by @tacshi in #994)

### Code editor, terminal and Markdown
- A long diff that fits its content scrolls without working through every line, a minified JSON file's diff shows in milliseconds instead of seconds, and a split diff with large fillers no longer drops frames. (#876, @hyoban, reported in #1300; #1335, @hyoban, reported in #1334; #1197, @overcat, reported in #1194)
- A patch's hunks are highlighted one by one, so a hunk ending in a block comment no longer colors the next one gray. (#1048, reported by @Binaryify in #1047)
- What an input method composes in the code editor is underlined. (#989, @goSeeFuture, reported in #987)
- The terminal no longer crashes reading its keyboard modes on Windows. (#1145, reported by @NriotHrreion in #1143)
- A program's synchronized update shows after a second even when it never ends it. (#1305, @daodao97)
- A full-screen terminal repaint takes less of the main thread, especially on Linux. (#1249, #1250, @Gaurav-Gosain)
- With `OptionAsAlt` on macOS, Option-E sends only Esc e. (#1004, reported by @lixw1994 in #1000)
- On macOS, CJK punctuation keeps its cells in the terminal, and holding a letter key repeats it. (#1084, @UncertaintyDeterminesYou4ndMe, reported in #1083; #1374)
- On Windows, what AltGr types reaches the terminal as text, so AltGr+Q types `@` on a German layout. (#1270, reported by @richardkfm in #1203)
- In Chinese and Japanese text, `**bold**` and `~~strikethrough~~` work without spaces around them, and a link ends at punctuation such as `，` or `。`. (#1057, @daodao97; #985, @baranwang, reported in #980)
- A link's hover underline is drawn whole instead of broken at descenders and slashes. (#1058, @daodao97)

### macOS
- A window resized from its view no longer keeps showing its old frame stretched. (#1045, reported by @liukun in #1044)
- A native UI window no longer stops drawing as it shows. (#1087)
- A tray's popover closes on a click on its icon. (#1059, reported by @gvenusleo in #1050)
- A page restored with `SessionState` keeps its scroll position as its window shows. (#1088)
- A window dragged by a drag region no longer shakes. (#1378)
- A Liquid Glass window that isn't `Transparent` shows what is behind it instead of solid gray. (#1355, reported by @contursofran in #1354)
- `mygo build` makes disk images on macOS 26.7.1. (#1353, reported by @rawat9 in #1352)
- Two-finger clicks open context menus even when security software re-posts them as left clicks. (#997, @roma-glushko)

### Windows
- Windows open and come back where they were on displays of different scales. (#1038, #1042)
- `Restore` leaves a visible maximized window maximized. (#1213, reported by @xiyaowong in #1212)
- Edit › Paste in a page pastes the clipboard's HTML, images and files, as Ctrl+V does. (#837, @echoVic)
- `LoadHTML` with an `about:blank` base URL shows its document. (#1031, reported by @ichts in #1029)
- Esc discards what the Japanese IME was composing. (#1032, reported by @ichts in #1030)
- Drawn menus show the last pixel of Chinese labels. (#1033, reported by @ands11 in #1028)
- An app that shows no page lets go of the hidden web view behind its cookie calls once idle, about 300 MB of WebView2 processes. (#1111, requested by @fondoger in #1109)
- On a maximized window with a material, the hovered Close button stays in the title bar. (#993, @Bush2021, reported in #992)
- Native UI windows report themselves to UI Automation as server-side providers. (#1007, @chengliang4810)

### Linux
- Native UI runs at the monitor's refresh rate instead of 60 Hz. (#1231, reported by @xishang0128 in #1219)
- Apps follow the desktop's dark style in GTK and in pages. (#1123, reported by @baka-world in #1120)
- On Wayland at a scale of 2, title buttons and shadows no longer show stale. (#1092, reported by @godcop in #1071)
- The tray shows the app's icon on KDE instead of a theme icon, and its tooltip updates on hosts such as quickshell. (#1216, reported by @Revincx in #1207; #1043, @ZHocean123)
- Subpixel text no longer looks pale with color fringes on old cairo. (#1027, reported by @buumotfa4djz6-tech in #1025)
- `srcdoc` iframes show when a page handles `OnWillNavigate`. (#1190, @baranwang, reported in #1189)
- Calling `WebViewAvailable` from a goroutine no longer makes the next page window abort. (#1384, reported by @r0k1s-i in #1383)
- One Ctrl+Z undoes a word committed by fcitx. (#988, @goSeeFuture, reported in #986)

### Windows and Linux
- A font registered under a family name of the app's own is found by that name alone, so faces of the same weight can be told apart. (#1232, reported by @fzn0x in #1224)

### Pages and plugins
- `fetch` keeps HttpOnly cookies from pages, honors an abort while a body is read, and keeps a request's body when `body: null` is passed with a `Request`. (#1235, #1345, @HiPanJin; #1105, @wiptgdxv)
- `WebSocket` rejects invalid subprotocols, and fails when the server doesn't acknowledge the one requested. (#1236, #1347, @HiPanJin)
- A channel passed to a call that failed before sending can be used again. (#1342, @HiPanJin)
- File watch grants no longer extend to a directory that replaced the file. (#1237, @HiPanJin)
- The generated TypeScript client matches the wire format for string-tagged number pointers and embedded fallbacks, and the generator reports names that would not compile instead of writing them. (#1243, #1252, #1234, #1294, #1341, #1337, @HiPanJin)

### Build, dev and updates
- `mygo dev` no longer hangs after the app exits when a child process keeps its output open. (#1024, @wiptgdxv)
- `mygo dev` rebuilds when `go.work` changes, and `mygo dev -web` picks up configuration changes. (#1338, #1343, @HiPanJin)
- `mygo build -platform web` keeps the old site when installing the new one fails, and bad `-platform` values fail at once. (#1196, #1238, @HiPanJin)
- Updates keep files' permissions. (#1340, @HiPanJin)
- `mygo build` still makes delta updates when the latest GitHub release is missing a platform's files. (#995, @echoVic)
- `mygo-automation wait` finds its text in a window that keeps drawing. (#1175, @HiPanJin)

## Website
- `/benchmarks` compares MyGo with GPUI, GPUI Fast and egui on the same four scenes, measured on a Mac. (#1311)
- The Showcase adds Tomoru, Quaver Astra, Meiro, us3ui, repo-hopper, RcloneDrive, ApiGo, Silverwing, Nodra, NanoDoll, Teebots, Screenpin and Sotto. (#979 @ACAne0320, #1018, #1061 @elianiva, #1094 @pteich, #1144 @younland, #1230 @yArna, #1332 @xijaja, #1370 @wiptgdxv, #1382 @doyuli)

## New Contributors
* @goSeeFuture made their first contribution in https://github.com/egoist/mygo/pull/988
* @roma-glushko made their first contribution in https://github.com/egoist/mygo/pull/997
* @hronro made their first contribution in https://github.com/egoist/mygo/pull/1013
* @wiptgdxv made their first contribution in https://github.com/egoist/mygo/pull/1024
* @ZHocean123 made their first contribution in https://github.com/egoist/mygo/pull/1043
* @elianiva made their first contribution in https://github.com/egoist/mygo/pull/1061
* @UncertaintyDeterminesYou4ndMe made their first contribution in https://github.com/egoist/mygo/pull/1084
* @wibus-wee made their first contribution in https://github.com/egoist/mygo/pull/1091
* @pteich made their first contribution in https://github.com/egoist/mygo/pull/1094
* @baka-world made their first contribution in https://github.com/egoist/mygo/pull/1119
* @puresky271 made their first contribution in https://github.com/egoist/mygo/pull/1125
* @HarriesCC made their first contribution in https://github.com/egoist/mygo/pull/1131
* @younland made their first contribution in https://github.com/egoist/mygo/pull/1144
* @kyya made their first contribution in https://github.com/egoist/mygo/pull/1149
* @1bit-JS made their first contribution in https://github.com/egoist/mygo/pull/1159
* @HiPanJin made their first contribution in https://github.com/egoist/mygo/pull/1175
* @yArna made their first contribution in https://github.com/egoist/mygo/pull/1230
* @Gaurav-Gosain made their first contribution in https://github.com/egoist/mygo/pull/1249
* @iFwu made their first contribution in https://github.com/egoist/mygo/pull/1229
* @hyoban made their first contribution in https://github.com/egoist/mygo/pull/876
* @xijaja made their first contribution in https://github.com/egoist/mygo/pull/1332
* @BABA983 made their first contribution in https://github.com/egoist/mygo/pull/1313
* @sugarbearr made their first contribution in https://github.com/egoist/mygo/pull/1356
* @Kuangcp made their first contribution in https://github.com/egoist/mygo/pull/1379
* @doyuli made their first contribution in https://github.com/egoist/mygo/pull/1382

**Full Changelog**: https://github.com/egoist/mygo/compare/v0.7.4...v0.8.0


# v0.7.4

https://github.com/egoist/mygo/releases/tag/v0.7.4

Published: 2026-11-01T06:05:02Z

## Highlights

- **Native UI windows open fully drawn.** On Windows, a native UI window stays out of sight until its first frame is ready, so it opens showing its content instead of a white flash. A window made with `Hidden: true` draws its first frame while hidden on Windows, macOS and Linux, so `w.OnReadyToShow(w.Show)` shows a splash screen or a settings window only once it's drawn. (#913, #915, reported by @Zbycn in #905)
- **Web views that scroll the native UI around them.** `WebViewOptions.ScrollToContainer` sends the wheel and touchpad over a web view to the native scroll container it sits in, on macOS, Windows and Linux. Use it for a page shown at its full height among native views, such as an article between a native header and its comments, or a message in a mail thread. (#943, with @everpcpc)
- **Another theme for part of a window.** `ui.Themed(c, theme)` builds its children with a theme of their own, so a dark window can hold a light document preview or a card that shows how a theme looks. (#932, requested by @Garfielk in #931)
- **An Edit menu that follows the focus.** In native UI windows, Undo, Redo, Cut, Copy, Paste, Delete and Select All are enabled only when the focused content can act on them, in the menu bar, popups and their shortcuts, on macOS, Windows and Linux. A password field can't be copied from, and a read-only one can't be pasted into. (#971, @liby)
- **Arch Linux packages.** `mygo build` writes a `PKGBUILD` next to each Linux archive, so `makepkg -si` installs the app with pacman, with its commands, desktop entry, file types and icons, as the Debian package does. (#934, requested by @xmengnet in #933)
- **Idle native UI windows on macOS use less power.** An idle window no longer wakes the app over a hundred times a second: wakeups drop from about 135 to 13 a second, and CPU from 0.8% to 0.3%. (#928, @daodao97)

## New

### Native UI
- `ui.DecimalInput` edits an exact decimal kept as a string, such as a price or an amount of money, with the same stepper as `NumberInput` and none of a float's rounding. It never rounds what the user types: too many decimals or a value out of range is marked as an error. (#937, requested by @CHEUKFUNGWU in #936)
- `Painter.FillRadialGradient` and `Element.RadialGradient` draw radial gradients, for glows, spotlights, vignettes and shaded spheres. (#910, requested by @Garfielk in #907)
- `Element.IgnoreInput` lets an element ignore the pointer and the keyboard while screen readers can still press it, such as a delete button that only shows on hover. (#927, requested by @Lynskylate in #926)
- `Calendar(c, &date).Browse()` pages through months without choosing a day, so a calendar in a popover closes only when the user picks one. (#903, requested by @tacshi in #898)
- `Element.Scrollbars(false)` hides a scroll container's bars for a carousel, a map or a panel with its own chrome. The wheel, the keyboard and assistive technology still scroll it. (#972, @chengliang4810)
- `PlaceholderColor` tints an input's hint, such as a warning or a "type DELETE to confirm". `CaretWidth` sets the caret's width, for a block cursor in a terminal-style input, a thicker caret in a large-print theme, or none in a preview. (#969, #972, @chengliang4810)
- `DottedUnderline` underlines text with dots, for links that mark hover the way browsers and editors do. (#969, @chengliang4810)
- `Context.Instant` tells a view that animates a value of its own to settle it at once, in the same frames as `DisableAnimations` settles everything else. (#969, @chengliang4810)
- `ui.Scrollbar` and `Painter.Scrollbar` give a view that scrolls itself, such as a custom list or canvas, the same scroll bars as a scroll container. (#949)
- Screen readers and voice control can correct the text of an element with its own text handling (`HandleInput`), such as a chat composer or a terminal, as they can in a `TextInput`. (#967, @chengliang4810)

### Markdown
- `Options.Anchor` opens a document at a heading, so an in-app docs reader can open `guide.md#install` from a search result or reopen where its reader left off. (#969, @chengliang4810)
- `Theme.LinkWeight` makes links bolder, so they read as links where color alone doesn't, as in a gray print theme or for a colorblind reader. (#969, @chengliang4810)

### Dialogs, notifications and updates
- `MessageOptions.DestructiveButtons` marks the buttons of a message that delete data or can't be undone, so they stand out on macOS and Linux. (#938, @liby)
- `Notification.ShowContext` stops waiting for the user to answer the notification permission prompt when its context ends, such as when the app quits with the prompt still open. (#952, @chengliang4810)
- `updater.AvailableVersion()` gives the version the last update check found, so a settings row or badge can say "Version 1.4 is available" even when the check finished before the UI was built. (#960, @chengliang4810)

### Windows
- `WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS` and `WEBVIEW2_USER_DATA_FOLDER` work for MyGo apps, so a test runner or an agent can reach an app's pages with `--remote-debugging-port`, as it can with other WebView2 apps. (#657, @echoVic)

### Build and distribution
- `linux.command` takes a list of names, so an app can add a short command, such as `ma` next to `my-app`. (#900, @overcat)
- `mygo build -skip-installer` builds for Windows without making the installer or downloading NSIS, for apps that ship their own installer or only the update archive. Apps installed earlier keep updating. (#902, requested by @suixinex in #897)

## Fixes

### UI and apps
- A grid child's percentages, such as `FillWidth()` and `FillHeight()`, are of its own cell, so a card in a two-column grid no longer covers its neighbour. (#909, reported by @Garfielk in #906)
- A `PopoverMenu` on an element scrolled partly out of view opens at the pointer. (#896, reported by @tacshi in #894)
- The scroll bars of the code editor, rich text and the terminal look like a scroll container's: they follow `Theme.ScrollbarWidth` and High Contrast's colors, and show while the pointer is over the view. (#944, #949)
- Delta updates of large apps finish building. (#941, reported by @tacshi in #939)

### Code editor and terminal
- Dragging a selection past the editor's left or right edge scrolls sideways. (#735, @xingkaixin)
- The wheel scrolls a diff with the pointer over a hunk header or an expand bar. (#935, @Zenyet)
- A diff as high as its content, and the old side of a split diff, show a horizontal scroll bar for long lines. (#947)
- A split diff's filler between hunks of very different lengths shows its hatching and no longer freezes scrolling. (#911, reported by @Binaryify in #904)
- The terminal draws block sextants, so QR codes from Expo and similar tools show. (#920, @shqingda)

### macOS
- The first click on a page in a window that isn't in front reaches the page, so buttons and links no longer take two clicks after switching back to the app. (#950, reported by @oxyuan in #946)
- On macOS 26 with several displays, a tray's flyout opens under the copy of the icon you clicked. (#942, @echoVic)
- Escape and Cmd+. choose a message's cancel button even when it's also the default, so one-button alerts, `Dialog.Error` and a page's `alert()` close with Escape. (#938, @liby)
- `mygo build` makes disk images on macOS 27. (#925, @patrickatrender)

### Windows
- Pages show the app's forced light or dark appearance from the start, without a flash of the system's. (#975)
- A native UI window shows its background, not black, along its edges while it shrinks. (#895)
- On Windows 10, a frameless window shows its content up to its edges, without a white line around it. (#918, reported by @Zbycn in #916)
- On Windows 11 at high scaling, menus no longer cut off the end of their longest label. (#923, reported by @Amamiya23 in #922)
- Pasting text copied from Notepad works with `clipboard.ReadData`. (#912)
- `mygo build` no longer fails when a virus scanner briefly holds a file it just wrote. (#953, @chengliang4810)
- The installer's "Create a desktop shortcut" box is in the installer's language, and apps can translate it with `mygo.desktopShortcut`. (#662, @echoVic)

## Website
- The Showcase adds Video-Downloader and Lumilo. (#917 @fazi-gondal, #919 @kotuke)

## New Contributors
* @overcat made their first contribution in https://github.com/egoist/mygo/pull/900
* @fazi-gondal made their first contribution in https://github.com/egoist/mygo/pull/917
* @kotuke made their first contribution in https://github.com/egoist/mygo/pull/919
* @shqingda made their first contribution in https://github.com/egoist/mygo/pull/920
* @patrickatrender made their first contribution in https://github.com/egoist/mygo/pull/925
* @daodao97 made their first contribution in https://github.com/egoist/mygo/pull/928
* @Zenyet made their first contribution in https://github.com/egoist/mygo/pull/935
* @liby made their first contribution in https://github.com/egoist/mygo/pull/938
* @everpcpc made their first contribution in https://github.com/egoist/mygo/pull/943
* @chengliang4810 made their first contribution in https://github.com/egoist/mygo/pull/960

**Full Changelog**: https://github.com/egoist/mygo/compare/v0.7.3...v0.7.4
