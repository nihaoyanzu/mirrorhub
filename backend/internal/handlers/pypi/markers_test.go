package pypi

import (
	"reflect"
	"testing"
)

func TestPythonVersionsFromFloor(t *testing.T) {
	got := pythonVersionsFromFloor("3.9")
	want := []string{"3.9", "3.10", "3.11", "3.12", "3.13"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("floor 3.9: got %v want %v", got, want)
	}
	got = pythonVersionsFromFloor("3.13")
	if !reflect.DeepEqual(got, []string{"3.13"}) {
		t.Fatalf("floor 3.13: got %v", got)
	}
	got = pythonVersionsFromFloor("3.14")
	if !reflect.DeepEqual(got, []string{"3.13"}) {
		t.Fatalf("floor clamp: got %v", got)
	}
}

func TestCPTagsFromFloor(t *testing.T) {
	tags := CPTagsFromFloor("3.11")
	want := []string{"cp311", "cp312", "cp313"}
	if !reflect.DeepEqual(tags, want) {
		t.Fatalf("got %v want %v", tags, want)
	}
}

func TestCPTagsFromVersions(t *testing.T) {
	tags := CPTagsFromVersions([]string{"3.10", "3.12"})
	want := []string{"cp310", "cp312"}
	if !reflect.DeepEqual(tags, want) {
		t.Fatalf("got %v want %v", tags, want)
	}
	// 去重
	tags = CPTagsFromVersions([]string{"3.10", "3.10", "3.12"})
	if !reflect.DeepEqual(tags, []string{"cp310", "cp312"}) {
		t.Fatalf("应去重: %v", tags)
	}
	// 空列表
	tags = CPTagsFromVersions(nil)
	if len(tags) != 0 {
		t.Fatalf("nil 应返回空: %v", tags)
	}
}

func TestEvalMarkerFloorUnion(t *testing.T) {
	// 测试版本列表：EvalMarker 对任一版本满足即纳入
	env := TargetEnv{Python: []string{"3.9", "3.10", "3.11", "3.12", "3.13"}, Platform: "linux"}
	if !EvalMarker(`python_version >= "3.12"`, env) {
		t.Fatal("版本列表中含 3.12+，应纳入")
	}
	if EvalMarker(`python_version < "3.9"`, env) {
		t.Fatal("低于所有版本的不应匹配")
	}
	if !EvalMarker(`python_version >= "3.9" and python_version < "3.14"`, env) {
		t.Fatal("and 组合应成立")
	}
	// 精确版本列表
	specific := TargetEnv{Python: []string{"3.10", "3.12"}, Platform: "linux"}
	if !EvalMarker(`python_version >= "3.10"`, specific) {
		t.Fatal("3.10 满足 >=3.10")
	}
	if EvalMarker(`python_version >= "3.13"`, specific) {
		t.Fatal("3.10 和 3.12 都不满足 >=3.13")
	}
}

func TestEvalMarkerExtrasAndPlatform(t *testing.T) {
	env := TargetEnv{Python: []string{"3.10"}, Platform: "linux"}
	if EvalMarker(`extra == "dev"`, env) {
		t.Fatal("extras 应排除")
	}
	if !EvalMarker(`extra == ''`, env) {
		t.Fatal("extra 空串应保留")
	}
	if EvalMarker(`sys_platform == "win32"`, env) {
		t.Fatal("linux 环境不应匹配 win32")
	}
	if !EvalMarker(`sys_platform == "linux"`, env) {
		t.Fatal("linux 应匹配")
	}
	win := TargetEnv{Python: []string{"3.10"}, Platform: "windows"}
	if !EvalMarker(`platform_system == "Windows"`, win) {
		t.Fatal("windows 归一化后应匹配 platform_system")
	}
}

func TestEvalMarkerMalformedNoPanic(t *testing.T) {
	env := TargetEnv{Python: []string{"3.9"}, Platform: "linux"}
	cases := []string{
		`python_version >>> "x"`,
		`(((broken`,
		`unknown_marker == "x"`,
		`python_version >= 3.9`, // 无引号，保守 true
	}
	for _, m := range cases {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("marker %q panic: %v", m, r)
				}
			}()
			_ = EvalMarker(m, env)
		}()
	}
}

func TestSplitReqMarker(t *testing.T) {
	req, marker := SplitReqMarker(`requests>=2.0; python_version >= "3.8"`)
	if req != "requests>=2.0" || marker == "" {
		t.Fatalf("got req=%q marker=%q", req, marker)
	}
	req, marker = SplitReqMarker("numpy")
	if req != "numpy" || marker != "" {
		t.Fatalf("无 marker: req=%q marker=%q", req, marker)
	}
}

func TestParseRequiresDist(t *testing.T) {
	meta := []byte("Name: pkg\nRequires-Dist: foo (>=1)\nRequires-Dist: bar; extra == 'dev'\n")
	got := ParseRequiresDist(meta)
	if len(got) != 2 || got[0] != "foo (>=1)" {
		t.Fatalf("got %#v", got)
	}
}
