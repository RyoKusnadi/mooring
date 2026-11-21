# Projects and files

A project is a folder on the computer running Mooring. Each project has its own panes, files,
changes and chats.

## Add a project

1. Click the "+" ("Add project") button beside "Projects" in the sidebar. The "Add project"
   dialog has three tabs.
2. To use a folder you already have, stay on "Open a folder". Type a path in "Folder on this
   computer" and click "Go", or click folders in the list. "Up one folder" goes back. Folders that
   are git repositories carry a "git repository" tag. Click "Open this folder".
3. To make a folder, pick "New folder". Choose the Parent folder (Browse and Hide folders show and
   hide the picker, and "Use this folder" confirms it), type a Folder name and click "Create
   folder".
4. To clone, pick "Clone a repository". Enter the Repository address, which starts with
   https://, http://, ssh://, git:// or file://. Mooring cannot ask for a password, so use a
   repository that needs none or a key or credential helper already set up. Choose the Parent folder.
   "Folder name (optional)" can stay empty to use the name from the address. Click "Clone
   repository".

![The Add project dialog on the Clone a repository tab, with an address typed in.](images/projects-add-project.png)

## Work with the project list

- Click a project to switch to it.
- Drag a row to reorder, or use the "…" menu on the row: Rename, Move up, Move down, and
  "Remove from list…". Removing only drops it from the list. Its files stay on disk.
- If the folder has gone, the project says "The folder of this project is missing". Click "Choose
  another folder" or "Remove project".

## The file tree

Click "Files" in the sidebar to see the tree of the open project.

- The "New file" and "New folder" buttons at the top add to the folder you last clicked, or to the project folder if you have not clicked one.
- A single click previews a file in a tab that the next click replaces. Press Enter or
  double-click to keep it open.
- Keys: Up and Down move, Home and End jump, Right expands a folder, Left collapses it, F2
  renames, M moves, Delete deletes.
- Hidden and ignored entries show dimmed, tagged "(hidden)" or "(ignored)". Links are tagged "(link
  leaves the project)" or "(broken link)".
- A long folder shows "Showing N of M."
- An empty project says "This project folder is empty. Use New file to add one."

### The row menu

Open it with the "…" button on a row, or right-click. Folders have "New file here" and "New folder
here" and "Upload files…". Files have "Download". Every row has "Move to…", "Duplicate", "Copy
path", "Copy relative path", "Rename…" and "Delete…".

![The row menu of the src folder: New file here, New folder here, Upload files, Move to, Duplicate, Copy path, Copy relative path, Rename and Delete.](images/projects-file-menu.png)

To rename a file or folder:

1. Open the row menu and click "Rename…", or select the row and press F2.
2. Type the new name in the box that appears and press Enter.

To move one:

1. Drag it onto a folder. Mooring moves it at once and shows "Moved X to Y." for 12 seconds.
2. Click "Undo" in that message to put it back.

Or open the row menu, click "Move to…", type part of a folder name and pick the folder.

If the new place already has that name, Mooring asks "X already exists in Y." Choose "Replace"
(the old file goes to the trash), "Keep both" or "Cancel".

### Delete and the trash

"Delete…" asks "Move X to the trash?" Click "Move to trash" or "Keep". Mooring has no undo for the
trash. Open the Trash in Finder to get a file back. Only moves have "Undo".

Where there is no trash (Linux), it asks "There is no trash to move it to. Delete X for good? This
cannot be undone." and the button reads "Delete permanently".

### Upload and download

To upload, drop files from Finder onto a folder, or use "Upload files…" in a folder's menu.
Folders cannot be dropped: Mooring says "Folders cannot be uploaded yet. Drop the files inside
it." A file over 100 MB is refused with "X is over 100 MB. Copy large files with a terminal."

To download, open a file's row menu and click "Download".

## The editor

Click a file to open it. The top shows the path and, for a file Mooring cannot change, a
"Read-only" tag. The editor has colours for the language, search, undo and, with a language server,
hover, completions and errors (see [Language servers](language-servers.md)).

To save: press Cmd+S (Ctrl+S elsewhere), or click "Save", which shows while the file has changes.
The state next to the path reads "Saving…", "Unsaved changes" or "Updated from disk". "Format"
shows when a language server can format the file.

Press Tab to indent. To leave the editor with the keyboard, press Esc, then Tab. Files over 2 MB
open without colours.

Some files do not open as text. A binary file says "This is a binary file, so it is not shown
here. Open it with another program." A file over 10 MB says "This file is over 10 MB, which is
more than the editor opens." Images show as pictures.

### Unsaved changes

- Closing a tab with edits asks "Close X without saving?" with "Save and close", "Discard changes"
  and "Keep editing".
- Closing or reloading the browser page warns while any file has unsaved edits.
- If the file changed on disk while you had edits, the editor says "This file changed on disk, and
  you have unsaved edits here." Click "Load the file from disk" to take the disk version, or
  "Overwrite the file on disk with my version" to keep yours.
- If you save and the disk version is newer than the one you opened, it says "Not saved. The file
  on disk is newer than the version you opened." and offers the same two buttons.
- If the file was deleted, it says "This file was deleted from disk. Save to create it again."

### Ask an agent about lines

Select lines and click "Ask the agent about these lines". The lines become an attachment in your
chat. See [Agent chats](agent-chats.md). The button reads "Select lines to ask the agent" until
you select something, and it works on local files only.

## Find a file or text

Press Alt+O or Cmd+P (Ctrl+P elsewhere; in a focused terminal on Linux, use Alt+O) to open "Find in project". The dialog has two toggles,
"Open file" and "Search text". Alt+F or Cmd+Shift+F (Ctrl+Shift+F elsewhere) opens it on "Search
text".

To open a file by name:

1. Open the dialog on "Open file".
2. Type part of the file name. Up and Down move through the list.
3. Press Enter to open the file.

To search text:

1. Switch to "Search text" and type the text to find in this project.
2. Optionally tick "Match case" or "Regex", or fill "Files" with a pattern such as `*.go` or
   `!*.test.ts` to include or leave out files.
3. Press Up and Down to move, Enter to open a match. Esc closes the results.

![Find in project on Search text: the word greet has five matching lines, with the matches highlighted.](images/projects-find-text.png)

It shows "Searching…", then "N lines". If there are too many matches it says "Showing the first N
lines. Narrow the search to see the rest." With none, "Nothing found for “q”." If files changed
since you searched, it says "Files changed since this search."

Next: [Terminals](terminals.md).
