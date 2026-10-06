package controller

import (
	"os"
	"testing"
)

func TestSubstitute(t *testing.T) {
	_ = os.Setenv("SC_TEST_A", "from-env")
	_ = os.Setenv("SC_TEST_EMPTY", "")
	defer func() {
		_ = os.Unsetenv("SC_TEST_A")
		_ = os.Unsetenv("SC_TEST_EMPTY")
	}()

	defaults := map[string]string{"SC_TEST_A": "overridden", "SC_TEST_B": "from-default"}
	cases := []struct{ in, want string }{
		{"$SC_TEST_A", "from-env"},
		{"${SC_TEST_A}", "from-env"},
		{"$SC_TEST_B", "from-default"},
		{"${SC_TEST_B:-fallback}", "from-default"},
		{"${SC_TEST_C:-fallback}", "fallback"},
		{"${SC_TEST_EMPTY:-fallback}", ""},
		{"a$SC_TEST_A-b", "afrom-env-b"},
		{"{http.request.header.X-Forwarded-For}", "{http.request.header.X-Forwarded-For}"},
		{"reverse_proxy http://$SC_TEST_B:$SC_TEST_C", "reverse_proxy http://from-default:"},
	}
	for _, c := range cases {
		if got := substitute(c.in, defaults); got != c.want {
			t.Errorf("%q: 期望 %q 得到 %q", c.in, c.want, got)
		}
	}
}

func TestParseOwner(t *testing.T) {
	uid, gid, err := parseOwner("65532:65532")
	if err != nil || uid != 65532 || gid != 65532 {
		t.Fatalf("65532:65532 -> %d %d %v", uid, gid, err)
	}
	uid, gid, err = parseOwner("33")
	if err != nil || uid != 33 || gid != 33 {
		t.Fatalf("33 -> %d %d %v", uid, gid, err)
	}
	for _, bad := range []string{"nonroot", "1:x", ":2"} {
		if _, _, err := parseOwner(bad); err == nil {
			t.Errorf("%q 应该报错", bad)
		}
	}
}

func TestFileMode(t *testing.T) {
	if got := fileMode("0644", 0); got != 0o644 {
		t.Errorf("0644 -> %v", got)
	}
	if got := fileMode("", 0o755); got != 0o755 {
		t.Errorf("空值应回退默认 -> %v", got)
	}
	if got := fileMode("abc", 0o600); got != 0o600 {
		t.Errorf("非法值应回退默认 -> %v", got)
	}
}
