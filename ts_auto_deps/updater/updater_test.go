package updater

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/bazelbuild/buildtools/build"
	"github.com/golang/protobuf/proto"

	arpb "github.com/bazelbuild/rules_typescript/ts_auto_deps/proto"
)

var (
	testTmpDir = os.Getenv("TEST_TMPDIR")
)

func mktmp(fn string, content []byte) (string, error) {
	p := fn
	if !filepath.IsAbs(p) {
		p = filepath.Join(testTmpDir, fn)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0777); err != nil {
		return "", err
	}
	return p, ioutil.WriteFile(p, content, 0666)
}

func TestReadBuild(t *testing.T) {
	p, err := mktmp("google3/foo/bar/BUILD", []byte(`
ts_library(name = 'a', srcs = ['b.ts'])
`))
	if err != nil {
		t.Fatal(err)
	}
	bld, err := readBUILD(context.Background(), filepath.Join(testTmpDir, "google3"), p)
	if err != nil {
		t.Fatal(err)
	}
	if bld.Path != "foo/bar/BUILD" {
		t.Errorf("bld.Path: got %s, expected %s", bld.Path, "foo/bar/BUILD")
	}
}

func TestGlobSources(t *testing.T) {
	for _, f := range []string{"a.ts", "a/b.ts", "c.tsx", "whatever", "foo.cpp", "d.d.ts", "._e.ts"} {
		if _, err := mktmp(f, []byte("// content")); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("../bazel-bin/symlink.d.ts", filepath.Join(testTmpDir, "symlink.d.ts")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("whatever", filepath.Join(testTmpDir, "whatever.d.ts")); err != nil {
		t.Fatal(err)
	}
	srcs, err := globSources(context.Background(), testTmpDir, []string{"ts", "tsx"})
	if err != nil {
		t.Fatal(err)
	}
	expected := srcSet(map[string]bool{
		"a.ts":          true,
		"c.tsx":         true,
		"d.d.ts":        true,
		"whatever.d.ts": true,
	})
	if !reflect.DeepEqual(srcs, expected) {
		t.Errorf("globSources: got %v, want %v", srcs, expected)
	}
}

func TestDetermineRuleType(t *testing.T) {
	tests := []struct {
		path   string
		source string
		rt     ruleType
	}{
		{"java/com/google/myapp/BUILD", "foo.ts", ruleTypeRegular},
		{"java/com/google/myapp/BUILD", "foo_test.ts", ruleTypeTest},
		{"java/com/google/myapp/BUILD", "foo_test.tsx", ruleTypeTest},

		{"java/com/google/testing/mytesttool/BUILD", "foo.ts", ruleTypeRegular},
		{"testing/mytesttool/BUILD", "foo.ts", ruleTypeRegular},
		{"testing/mytesttool/BUILD", "foo_test.ts", ruleTypeTest},
		{"testing/mytesttool/BUILD", "foo_test.ts", ruleTypeTest},
	}
	for _, tst := range tests {
		rt := determineRuleType(tst.path, tst.source)
		if rt != tst.rt {
			t.Errorf("determineRuleType(%q, %q): got %v, expected %v", tst.path, tst.source, rt, tst.rt)
		}
	}
}

func parseReport(t *testing.T, input string) *arpb.DependencyReport {
	report := &arpb.DependencyReport{}
	if err := proto.UnmarshalText(input, report); err != nil {
		t.Error(err)
	}
	return report
}

func TestBazelAnalyzeError(t *testing.T) {
	bld, err := build.ParseBuild("rules/BUILD", []byte(`
ts_library(
	name = "firstrule",
	srcs = [],
)
ts_library(
	name = "secondrule",
	srcs = [],
)
	`))
	if err != nil {
		t.Fatal(err)
	}
	mockAnalyze := func(_ string, targets []string) ([]byte, []byte, error) {
		data, err := proto.Marshal(&arpb.AnalyzeResult{
			DependencyReport: []*arpb.DependencyReport{&arpb.DependencyReport{
				Rule: proto.String("//rules:firstrule"),
			}},
		})
		return data, []byte(`Here's the actual error`), err
	}
	upd := &Updater{}
	upd.bazelAnalyze = mockAnalyze
	report, err := upd.runBazelAnalyze("firstrule/BUILD", bld, bld.Rules("ts_library"))
	if err == nil {
		t.Fatalf("expected an error, got a report: %v", report)
	}
	expected := `parsing reports failed (1 reports for [//rules:firstrule //rules:secondrule]):
Here's the actual error`
	if err.Error() != expected {
		t.Errorf("runBazelAnalyze: got %q, expected %q", err.Error(), expected)
	}
}

func TestUpdateDeps(t *testing.T) {
	report := parseReport(t, `
			rule: "//foo:bar"
			unnecessary_dependency: "//unnecessary_dep"
			missing_dependency_group: {
				dependency: "//missing_dep"
			}
			missing_source_file: "missing_file.ts"`)

	tests := []struct {
		name    string
		before  string
		after   string
		changed bool
	}{
		{
			"Add missing dependency",
			`ts_library(
					name = "bar",
					deps = [],
			)`,
			`ts_library(
					name = "bar",
					deps = ["//missing_dep"],
			)`,
			true,
		},
		{
			"Remove + Add dependency",
			`ts_library(
					name = "bar",
					deps = ["//unnecessary_dep"],
			)`,
			`ts_library(
					name = "bar",
					deps = ["//missing_dep"],
			)`,
			true,
		},
		{
			"Remove nonexistent dep (e.g. due to macro)",
			`ts_library(
					name = "bar",
					deps = ["//missing_dep"],
			)`,
			`ts_library(
					name = "bar",
					deps = ["//missing_dep"],
			)`,
			false, // Unchanged!
		},
		{
			"Remove nonexistent src",
			`ts_library(
					name = "bar",
					srcs = ["hello.ts"],
					deps = ["//missing_dep"],
			)`,
			`ts_library(
					name = "bar",
					srcs = ["hello.ts"],
					deps = ["//missing_dep"],
			)`,
			false, // Unchanged!
		},
	}
	for _, tst := range tests {
		bld, err := build.ParseBuild("foo/BUILD", []byte(tst.before))
		if err != nil {
			t.Errorf("parse %s failed: %s in %s", tst.name, err, tst.before)
		}
		bldAft, err := build.ParseBuild("foo/BUILD", []byte(tst.after))
		if err != nil {
			t.Errorf("parse %s after failed: %s", tst.name, err)
		}
		changed, err := updateDeps(bld, []*arpb.DependencyReport{report})
		if err != nil {
			t.Errorf("update %s failed: %s", tst.name, err)
		}
		updated := string(build.Format(bld))
		after := string(build.Format(bldAft))
		if updated != after {
			t.Errorf("update(%s), got:\n%s\n\nexpected:\n%s", tst.name, updated, after)
		}
		if changed != tst.changed {
			t.Errorf("changed(%s), got %t, expected %t", tst.name, changed, tst.changed)
		}
	}
}

func TestDottedCall(t *testing.T) {
	// Repro for a crash, b/35389044
	buildText := `foo.bar("baz")`
	bld, err := build.ParseBuild("test", []byte(buildText))
	if err != nil {
		t.Error(err)
	}
	removeUnusedLoad(bld, "ignored")
}

func TestFilterPaths(t *testing.T) {
	tests := []struct {
		in       []string
		expected []string
	}{
		{[]string{"foo/bar.txt", "foo/baz.ts"}, []string{"foo"}},
		{[]string{"bam.ts"}, []string{"."}},
		{[]string{"foo/BUILD"}, []string{"foo"}},
		{[]string{"r/foo.tsx"}, []string{"r"}},
		{[]string{"../../x.ts"}, []string{"../.."}},
		{[]string{"a.txt", "foo/b.txt"}, []string(nil)},
	}
	for _, tst := range tests {
		res := FilterPaths(tst.in)
		if !reflect.DeepEqual(res, tst.expected) {
			t.Errorf("FilterPaths(%v): got %v, expected %v", tst.in, res, tst.expected)
		}
	}
}

func TestAddDep(t *testing.T) {
	tests := []struct {
		buildFile string
		newDep    string
		expectAdd bool
	}{
		{`ts_library(name = "lib", deps = ["//a", "//b", "//c"])`, "//b", false},
		{`ts_library(name = "lib", deps = ["//a", "//b", "//c"])`, "//d", true},
		{`ts_library(name = "lib", deps = ["//a", ":b", "//c"])`, ":b", false},
		{`ts_library(name = "lib", deps = ["//a", ":b", "//c"])`, "//buildloc:b", false},
		{`ts_library(name = "lib", deps = ["//a", "//buildloc:b", "//c"])`, ":b", false},
		{`ts_library(name = "lib", deps = ["//a", "//other:b", "//c"])`, ":b", true},
		{`ts_library(name = "lib", deps = ["//a", "//other:b", "//c"])`, "//a:a", false},
	}
	for _, tst := range tests {
		bld, err := build.ParseBuild("buildloc/BUILD", []byte(tst.buildFile))
		if err != nil {
			t.Fatalf("parse failure: %s - %v", tst.buildFile, err)
		}
		res := addDep(bld, bld.Rules("ts_library")[0], tst.newDep)
		if res != tst.expectAdd {
			t.Errorf("addDep(%s, %s): got %v, expected %v", tst.buildFile, tst.newDep, res, tst.expectAdd)
		}
	}
}

func TestUpdateWebAssets(t *testing.T) {
	ctx := context.Background()
	bld, err := build.ParseBuild("foo/bar/BUILD",
		[]byte(`ng_module(name = "m", assets = [":rule", "gone.html"])`))
	if err != nil {
		t.Fatalf("parse failure: %v", err)
	}
	testHTML, err := mktmp("google3/foo/bar/test.html", []byte(`<p>test</p>`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(testHTML); err != nil {
			t.Error(err)
		}
	}()
	testCSS, err := mktmp("google3/foo/bar/test.css", []byte(`.test {}`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(testCSS); err != nil {
			t.Error(err)
		}
	}()
	absolutBuildPath := filepath.Join(filepath.Dir(testCSS), "BUILD")
	changed, err := updateWebAssets(ctx, absolutBuildPath, bld)
	if !changed {
		t.Errorf("expected BUILD to be changed")
	}
	if err != nil {
		t.Fatal(err)
	}
	data := string(build.Format(bld))
	expected := `ng_module(
    name = "m",
    assets = [
        ":rule",
        "test.css",
        "test.html",
    ],
)
`
	if data != expected {
		t.Errorf("build file mismatch, got %s, expected %s", data, expected)
	}
}

func TestWebAssetReferredByColon(t *testing.T) {
	ctx := context.Background()
	bld, err := build.ParseBuild("foo/bar/BUILD",
		[]byte(`ng_module(name = "m", assets = [":colon.html"])`))
	if err != nil {
		t.Fatalf("parse failure: %v", err)
	}
	colon, err := mktmp("google3/foo/bar/colon.html", []byte(`<p>test</p>`))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(colon); err != nil {
			t.Error(err)
		}
	}()
	absolutBuildPath := filepath.Join(filepath.Dir(colon), "BUILD")
	changed, err := updateWebAssets(ctx, absolutBuildPath, bld)
	if changed {
		t.Errorf("expected no BUILD change, got %s", string(build.Format(bld)))
	}
}

func TestAbsoluteBazelTarget(t *testing.T) {
	bld := &build.File{Path: "foo/bar/BUILD", Build: true}
	tests := []struct{ target, expected string }{
		{"//foo/bar:bar", "//foo/bar:bar"},
		{":bar", "//foo/bar:bar"},
		{"bar", "//foo/bar:bar"},
		{"//foo/bar", "//foo/bar:bar"},
	}
	for _, tst := range tests {
		abs := AbsoluteBazelTarget(bld, tst.target)
		if abs != tst.expected {
			t.Errorf("AbsoluteBazelTarget(%q): got %q, expected %q", tst.target, abs, tst.expected)
		}
	}
}

func TestFindBUILDFileCacheOnError(t *testing.T) {
	ctx := context.Background()
	cache := make(map[string]*build.File)
	p, err := mktmp("google3/pkg/file", []byte(""))
	if err != nil {
		t.Fatal(err)
	}
	g3root := filepath.Dir(filepath.Dir(p))
	if filepath.Base(g3root) != "google3" {
		t.Errorf("g3root should be called google3, got %q", g3root)
	}
	// No BUILD file was created in the file system so FindBUILDFile should
	// return an error.
	if _, err = FindBUILDFile(ctx, cache, g3root, "pkg"); err == nil {
		t.Fatalf("returned no error, expected some error to occur")
	}
	if _, ok := cache["pkg"]; ok {
		t.Fatalf("cache contained BUILD file for package")
	}
}
