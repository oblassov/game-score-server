package filesystem

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"

	"github.com/oblassov/game-score-server/internal/engine"
)

type PlayerStore struct {
	database *json.Encoder
	league   engine.League
	path     string
	lock     sync.RWMutex
	// lockFile is the "<db>.lock" sidecar that flock serialises stores on; the
	// db file itself can't carry the lock because every write renames a new
	// inode over it.
	lockFile *os.File
}

func PlayerStoreFromFile(path string) (*PlayerStore, func() error, error) {

	db, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return nil, nil, fmt.Errorf("problem opening %s: %w", path, err)
	}

	store, err := NewPlayerStore(db)
	if err != nil {
		return nil, nil, errors.Join(
			fmt.Errorf("problem creating file system store: %w", err),
			db.Close(),
		)
	}

	closeFunc := func() error {
		var errs []error
		if closeErr := db.Close(); closeErr != nil {
			errs = append(errs, fmt.Errorf("couldn't close %s: %w", path, closeErr))
		}
		if closeErr := store.Close(); closeErr != nil {
			errs = append(errs, closeErr)
		}

		return errors.Join(errs...)
	}

	return store, closeFunc, nil
}

func NewPlayerStore(file *os.File) (*PlayerStore, error) {

	lockPath := file.Name() + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("problem opening lock file %s: %w", lockPath, err)
	}

	err = initializePlayerDBFile(file)

	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("problem initializing player db file: %w", err),
			lockFile.Close(),
		)
	}

	league, err := engine.NewLeague(file)

	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("problem loading player store from file %s: %w", file.Name(), err),
			lockFile.Close(),
		)
	}

	return &PlayerStore{
		database: json.NewEncoder(&tape{file}),
		league:   league,
		path:     file.Name(),
		lockFile: lockFile,
	}, nil

}

// Close releases the store's lock file. It does not close the db file passed
// to NewPlayerStore.
func (f *PlayerStore) Close() error {
	if err := f.lockFile.Close(); err != nil {
		return fmt.Errorf("couldn't close lock file %s: %w", f.lockFile.Name(), err)
	}

	return nil
}

func initializePlayerDBFile(file *os.File) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("couldn't set an offset in a file: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("problem getting file info from file %s, %w", file.Name(), err)
	}

	blank := info.Size() == 0
	if !blank {
		data, readErr := io.ReadAll(file)
		if readErr != nil {
			return fmt.Errorf("couldn't read file %s: %w", file.Name(), readErr)
		}

		blank = len(bytes.TrimSpace(data)) == 0
		if blank {
			if err := file.Truncate(0); err != nil {
				return fmt.Errorf("couldn't truncate blank file %s: %w", file.Name(), err)
			}
		}
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("couldn't set an offset in a file: %w", err)
	}

	if blank {
		if _, err := file.WriteString("[]"); err != nil {
			return fmt.Errorf("couldn't write string: %w", err)
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("couldn't set an offset in a file when size is 0: %w", err)
		}
	}

	return nil
}

func (f *PlayerStore) GetLeague() engine.League {
	f.lock.RLock()
	defer f.lock.RUnlock()

	league := f.currentLeague()
	sort.Slice(league, func(i, j int) bool {
		if league[i].Wins != league[j].Wins {
			return league[i].Wins > league[j].Wins
		}
		return league[i].Name < league[j].Name
	})

	return league
}

func (f *PlayerStore) GetPlayerScore(name string) int {
	f.lock.RLock()
	defer f.lock.RUnlock()

	league := f.currentLeague()
	player := league.Find(name)

	if player != nil {
		return player.Wins
	}

	return 0
}

// currentLeague returns a copy of the league as it is on disk, so wins written
// by another store show up. The caller must hold f.lock. If the file can't be
// read it falls back to the last league this store read or wrote successfully.
func (f *PlayerStore) currentLeague() engine.League {
	league, err := f.readLeagueShared()
	if err != nil {
		league = f.league
	}

	return append(engine.League(nil), league...)
}

// readLeagueShared reads the db while holding the sidecar flock in shared mode.
func (f *PlayerStore) readLeagueShared() (engine.League, error) {
	if err := lockShared(f.lockFile); err != nil {
		return nil, fmt.Errorf("couldn't lock the db: %w", err)
	}
	defer f.unlockFile()

	return f.readLeague()
}

// unlockFile drops the sidecar flock. A failure is harmless: closing the lock
// file releases it, and the next lock call reports a broken descriptor.
func (f *PlayerStore) unlockFile() {
	if err := unlock(f.lockFile); err != nil {
		return
	}
}

func (f *PlayerStore) RecordWin(name string) error {
	f.lock.Lock()
	defer f.lock.Unlock()

	if err := lockExclusive(f.lockFile); err != nil {
		return fmt.Errorf("couldn't lock the db: %w", err)
	}
	defer f.unlockFile()

	// Another store may have rewritten the file since we last read it, so
	// start from what is on disk rather than from the cached league.
	league, err := f.readLeague()
	if err != nil {
		return fmt.Errorf("couldn't re-read the db: %w", err)
	}

	player := league.Find(name)

	if player != nil {
		player.Wins++
	} else {
		league = append(league, engine.Player{Name: name, Wins: 1})
	}

	if err := f.database.Encode(league); err != nil {
		return fmt.Errorf("couldn't encode the db: %w", err)
	}

	f.league = league

	return nil
}

func (f *PlayerStore) readLeague() (engine.League, error) {
	// tape.Write renames a new file over path, so the held *os.File points at
	// a stale inode; read by path instead.
	data, err := os.ReadFile(f.path)
	if err != nil {
		return nil, fmt.Errorf("couldn't read %s: %w", f.path, err)
	}

	// NewPlayerStore seeds a blank file with an empty league, so a blank file
	// here means it was emptied after that (e.g. mid-rewrite by another
	// process); refuse to overwrite it.
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, fmt.Errorf("couldn't load league from %s: file is empty", f.path)
	}

	league, err := engine.NewLeague(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("couldn't load league from %s: %w", f.path, err)
	}

	return league, nil
}
