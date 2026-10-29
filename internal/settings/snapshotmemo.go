package settings

// A Memo answers one frame's settings value.
//
// `Store.Snapshot` is not a field read: it deep-copies the whole value, and
// `clone` allocates a fresh map for the carried-through keys whenever there
// are any (`store.go`, `clone`). Surfaces read the settings from every row
// they draw — a row's tokens, its glyphs, the words it says — so asking once
// per row was a measurable share of a frame: the allocation profile of
// `BenchmarkGeneralFrame` put `settings.clone` at 9.31% of the objects, and
// instrumented counts of `Store.Snapshot` per frame were 53 on the
// Integrations page, 15 on General and 5 on the terminal surface (P11-co).
//
// The key is exact: the store's identity and its version. `Store.Version`
// documents "same version, same value", and both of `Update`'s endings — the
// successful write and the rollback of a refused one — bump it, so a memo can
// never serve a value the store no longer holds, and a write that the file
// refused is read back as the value that was confirmed. Two stores both begin
// at version zero, so the identity is part of the key rather than the version
// alone.
//
// A Memo belongs to one reader (an App field): it is not safe for concurrent
// use, and it keeps the last value it handed out.
type Memo struct {
	store   *Store
	version uint64
	val     Settings
	valid   bool
	built   uint64
}

// Get returns this reader's settings: the value it handed out last while the
// store has not moved, and a fresh snapshot otherwise. A nil store answers the
// zero value, which is what a surface with no settings to read draws with.
func (m *Memo) Get(store *Store) Settings {
	if store == nil {
		return Settings{}
	}
	version := store.Version()
	if m.valid && m.store == store && m.version == version {
		return m.val
	}
	m.val, m.store, m.version, m.valid = store.Snapshot(), store, version, true
	m.built++
	return m.val
}

// Builds counts the snapshots the memo took, so a test can see that a frame
// copies the settings once however many rows ask for them.
func (m *Memo) Builds() uint64 { return m.built }
