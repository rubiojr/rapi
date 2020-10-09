package restic_test

import (
	"io/ioutil"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/rubiojr/rapi/restic"
	rtest "github.com/rubiojr/rapi/test"
)

func BenchmarkNodeFillUser(t *testing.B) {
	tempfile, err := ioutil.TempFile("", "restic-test-temp-")
	if err != nil {
		t.Fatal(err)
	}

	fi, err := tempfile.Stat()
	if err != nil {
		t.Fatal(err)
	}

	path := tempfile.Name()

	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		restic.NodeFromFileInfo(path, fi)
	}

	rtest.OK(t, tempfile.Close())
	rtest.RemoveAll(t, tempfile.Name())
}

func BenchmarkNodeFromFileInfo(t *testing.B) {
	tempfile, err := ioutil.TempFile("", "restic-test-temp-")
	if err != nil {
		t.Fatal(err)
	}

	fi, err := tempfile.Stat()
	if err != nil {
		t.Fatal(err)
	}

	path := tempfile.Name()

	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		_, err := restic.NodeFromFileInfo(path, fi)
		if err != nil {
			t.Fatal(err)
		}
	}

	rtest.OK(t, tempfile.Close())
	rtest.RemoveAll(t, tempfile.Name())
}

func parseTime(s string) time.Time {
	t, err := time.Parse("2006-01-02 15:04:05.999", s)
	if err != nil {
		panic(err)
	}

	return t.Local()
}

var nodeTests = []restic.Node{
	{
		Name:       "testFile",
		Type:       "file",
		Content:    restic.IDs{},
		UID:        uint32(os.Getuid()),
		GID:        uint32(os.Getgid()),
		Mode:       0604,
		ModTime:    parseTime("2015-05-14 21:07:23.111"),
		AccessTime: parseTime("2015-05-14 21:07:24.222"),
		ChangeTime: parseTime("2015-05-14 21:07:25.333"),
	},
	{
		Name:       "testSuidFile",
		Type:       "file",
		Content:    restic.IDs{},
		UID:        uint32(os.Getuid()),
		GID:        uint32(os.Getgid()),
		Mode:       0755 | os.ModeSetuid,
		ModTime:    parseTime("2015-05-14 21:07:23.111"),
		AccessTime: parseTime("2015-05-14 21:07:24.222"),
		ChangeTime: parseTime("2015-05-14 21:07:25.333"),
	},
	{
		Name:       "testSuidFile2",
		Type:       "file",
		Content:    restic.IDs{},
		UID:        uint32(os.Getuid()),
		GID:        uint32(os.Getgid()),
		Mode:       0755 | os.ModeSetgid,
		ModTime:    parseTime("2015-05-14 21:07:23.111"),
		AccessTime: parseTime("2015-05-14 21:07:24.222"),
		ChangeTime: parseTime("2015-05-14 21:07:25.333"),
	},
	{
		Name:       "testSticky",
		Type:       "file",
		Content:    restic.IDs{},
		UID:        uint32(os.Getuid()),
		GID:        uint32(os.Getgid()),
		Mode:       0755 | os.ModeSticky,
		ModTime:    parseTime("2015-05-14 21:07:23.111"),
		AccessTime: parseTime("2015-05-14 21:07:24.222"),
		ChangeTime: parseTime("2015-05-14 21:07:25.333"),
	},
	{
		Name:       "testDir",
		Type:       "dir",
		Subtree:    nil,
		UID:        uint32(os.Getuid()),
		GID:        uint32(os.Getgid()),
		Mode:       0750 | os.ModeDir,
		ModTime:    parseTime("2015-05-14 21:07:23.111"),
		AccessTime: parseTime("2015-05-14 21:07:24.222"),
		ChangeTime: parseTime("2015-05-14 21:07:25.333"),
	},
	{
		Name:       "testSymlink",
		Type:       "symlink",
		LinkTarget: "invalid",
		UID:        uint32(os.Getuid()),
		GID:        uint32(os.Getgid()),
		Mode:       0777 | os.ModeSymlink,
		ModTime:    parseTime("2015-05-14 21:07:23.111"),
		AccessTime: parseTime("2015-05-14 21:07:24.222"),
		ChangeTime: parseTime("2015-05-14 21:07:25.333"),
	},

	// include "testFile" and "testDir" again with slightly different
	// metadata, so we can test if CreateAt works with pre-existing files.
	{
		Name:       "testFile",
		Type:       "file",
		Content:    restic.IDs{},
		UID:        uint32(os.Getuid()),
		GID:        uint32(os.Getgid()),
		Mode:       0604,
		ModTime:    parseTime("2005-05-14 21:07:03.111"),
		AccessTime: parseTime("2005-05-14 21:07:04.222"),
		ChangeTime: parseTime("2005-05-14 21:07:05.333"),
	},
	{
		Name:       "testDir",
		Type:       "dir",
		Subtree:    nil,
		UID:        uint32(os.Getuid()),
		GID:        uint32(os.Getgid()),
		Mode:       0750 | os.ModeDir,
		ModTime:    parseTime("2005-05-14 21:07:03.111"),
		AccessTime: parseTime("2005-05-14 21:07:04.222"),
		ChangeTime: parseTime("2005-05-14 21:07:05.333"),
	},
}

func AssertFsTimeEqual(t *testing.T, label string, nodeType string, t1 time.Time, t2 time.Time) {
	var equal bool

	// Go currently doesn't support setting timestamps of symbolic links on darwin and bsd
	if nodeType == "symlink" {
		switch runtime.GOOS {
		case "darwin", "freebsd", "openbsd", "netbsd":
			return
		}
	}

	switch runtime.GOOS {
	case "darwin":
		// HFS+ timestamps don't support sub-second precision,
		// see https://en.wikipedia.org/wiki/Comparison_of_file_systems
		diff := int(t1.Sub(t2).Seconds())
		equal = diff == 0
	default:
		equal = t1.Equal(t2)
	}

	rtest.Assert(t, equal, "%s: %s doesn't match (%v != %v)", label, nodeType, t1, t2)
}

func parseTimeNano(t testing.TB, s string) time.Time {
	// 2006-01-02T15:04:05.999999999Z07:00
	ts, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatalf("error parsing %q: %v", s, err)
	}
	return ts
}

func TestFixTime(t *testing.T) {
	// load UTC location
	utc, err := time.LoadLocation("")
	if err != nil {
		t.Fatal(err)
	}

	var tests = []struct {
		src, want time.Time
	}{
		{
			src:  parseTimeNano(t, "2006-01-02T15:04:05.999999999+07:00"),
			want: parseTimeNano(t, "2006-01-02T15:04:05.999999999+07:00"),
		},
		{
			src:  time.Date(0, 1, 2, 3, 4, 5, 6, utc),
			want: parseTimeNano(t, "0000-01-02T03:04:05.000000006+00:00"),
		},
		{
			src:  time.Date(-2, 1, 2, 3, 4, 5, 6, utc),
			want: parseTimeNano(t, "0000-01-02T03:04:05.000000006+00:00"),
		},
		{
			src:  time.Date(12345, 1, 2, 3, 4, 5, 6, utc),
			want: parseTimeNano(t, "9999-01-02T03:04:05.000000006+00:00"),
		},
		{
			src:  time.Date(9999, 1, 2, 3, 4, 5, 6, utc),
			want: parseTimeNano(t, "9999-01-02T03:04:05.000000006+00:00"),
		},
	}

	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			res := restic.FixTime(test.src)
			if !res.Equal(test.want) {
				t.Fatalf("wrong result for %v, want:\n  %v\ngot:\n  %v", test.src, test.want, res)
			}
		})
	}
}
