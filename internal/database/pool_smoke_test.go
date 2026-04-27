package database

import (
"path/filepath"
"testing"

"github.com/gateixeira/live-actions/pkg/logger"
)

func TestInitDB_PragmasAppliedToBothPools(t *testing.T) {
logger.InitLogger("error")
dir := t.TempDir()
w, r, err := InitDB(filepath.Join(dir, "t.db"))
if err != nil {
t.Fatalf("InitDB: %v", err)
}
defer w.Close()
defer r.Close()

cases := map[string]string{
"journal_mode": "wal",
"busy_timeout": "5000",
"synchronous":  "1", // NORMAL
"foreign_keys": "1",
}
for p, want := range cases {
var wv, rv string
if err := w.QueryRow("PRAGMA " + p).Scan(&wv); err != nil {
t.Fatalf("write PRAGMA %s: %v", p, err)
}
if err := r.QueryRow("PRAGMA " + p).Scan(&rv); err != nil {
t.Fatalf("read PRAGMA %s: %v", p, err)
}
if wv != want {
t.Errorf("write PRAGMA %s = %q, want %q", p, wv, want)
}
if rv != want {
t.Errorf("read PRAGMA %s = %q, want %q", p, rv, want)
}
}
}
