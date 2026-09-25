package main

import "testing"

func TestArchiveModeRequiresExplicitReviewedBackend(t *testing.T) {
	for _, c := range []config{
		{},
		{ArchiveMode: "local", ArchiveDirectory: "/tmp/archive"},
		{ArchiveMode: "local", ArchiveDirectory: ubuntuArchiveDirectory, R2Bucket: "unexpected"},
		{ArchiveMode: "r2", ArchiveDirectory: ubuntuArchiveDirectory},
	} {
		if _, err := selectedArchive(c); err == nil {
			t.Fatalf("accepted invalid archive configuration: %+v", c)
		}
	}
}
