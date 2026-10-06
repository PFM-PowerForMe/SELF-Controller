package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	openpgp "github.com/ProtonMail/go-crypto/openpgp/v2"
)

func makeTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	big := make([]byte, 300000)
	for i := range big {
		big[i] = byte(i * 7)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.bin"), big, 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "c.txt"), []byte("nested\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readArchive(t *testing.T, path string) map[string][]byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gzReader, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gzReader.Close()
	tarReader := tar.NewReader(gzReader)
	files := map[string][]byte{}
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		body, err := io.ReadAll(tarReader)
		if err != nil {
			t.Fatal(err)
		}
		files[header.Name] = body
	}
	return files
}

func TestArchiveRoundTrip(t *testing.T) {
	dir := makeTestDir(t)
	archive := filepath.Join(t.TempDir(), "test.tar.gz")
	if err := writeArchive(dir, archive); err != nil {
		t.Fatal(err)
	}
	if err := verifyArchive(archive); err != nil {
		t.Fatal(err)
	}

	files := readArchive(t, archive)
	if len(files) != 3 {
		t.Fatalf("归档里应有 3 个文件, 实际 %d 个: %v", len(files), files)
	}
	for _, name := range []string{"a.txt", "b.bin", filepath.Join("sub", "c.txt")} {
		want, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		got, ok := files[name]
		if !ok {
			t.Fatalf("归档里没有 %s", name)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s 内容不一致: %d 字节 vs 原 %d 字节", name, len(got), len(want))
		}
	}
}

func TestVerifyArchiveDetectsCorruption(t *testing.T) {
	dir := makeTestDir(t)
	archive := filepath.Join(t.TempDir(), "test.tar.gz")
	if err := writeArchive(dir, archive); err != nil {
		t.Fatal(err)
	}
	if err := verifyArchive(archive); err != nil {
		t.Fatalf("未改动的归档应该通过自检: %v", err)
	}

	raw, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)/2] ^= 0xff
	if err := os.WriteFile(archive, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := verifyArchive(archive); err == nil {
		t.Error("被改坏的归档应该自检失败")
	}
}

func TestEncryptArchiveReadableByRecipient(t *testing.T) {
	entity, err := openpgp.NewEntity("selftest", "", "selftest@example.com", nil)
	if err != nil {
		t.Fatal(err)
	}

	dir := makeTestDir(t)
	archive := filepath.Join(t.TempDir(), "test.tar.gz")
	if err := writeArchive(dir, archive); err != nil {
		t.Fatal(err)
	}
	encrypted := filepath.Join(t.TempDir(), "test.tar.gz.gpg")
	if err := encryptArchive(archive, encrypted, openpgp.EntityList{entity}); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	message, err := openpgp.ReadMessage(f, openpgp.EntityList{entity}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(message.UnverifiedBody)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("解密结果与归档不一致: 得到 %d 字节, 期望 %d 字节", len(got), len(want))
	}
	if err := verifyArchive(encrypted); err == nil {
		t.Error("加密文件不该被当成归档读")
	}
}
