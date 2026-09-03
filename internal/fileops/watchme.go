package fileops

// WatchMeDirName is the folder inside the user's replay directory where the
// dashboard stages a replay for the user to open in StarCraft. It starts with
// "000_" so it sorts above other folders (and folders sort above files) in
// StarCraft's replay browser. Replays under it are never part of the corpus.
const WatchMeDirName = "000_screpdb_watch_me"
