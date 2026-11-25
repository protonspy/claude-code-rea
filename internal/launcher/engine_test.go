package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGhidra returns a directory declaring Ghidra version, needing JDK javaMin ("" for none).
func fakeGhidra(t *testing.T, version, javaMin string) string {
	t.Helper()
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "Ghidra"), 0o755)
	props := "#Fri Oct 16 12:20:39 UTC 2026\napplication.name=Ghidra\napplication.java.max=\napplication.java.min=" + javaMin + "\napplication.version=" + version + "\n"
	if err := os.WriteFile(filepath.Join(dir, "Ghidra", "application.properties"), []byte(props), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// fakeJDK returns a directory whose release file declares JAVA_VERSION version.
func fakeJDK(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	release := "IMPLEMENTOR=\"Eclipse Adoptium\"\nJAVA_VERSION=\"" + version + "\"\nOS_NAME=\"Windows\"\n"
	if err := os.WriteFile(filepath.Join(dir, "release"), []byte(release), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestREAAdmitsGhidraFrom12Point2OnTheSameLine(t *testing.T) {
	for version, want := range map[string]bool{
		"12.2": true, "12.2.1": true, "12.3": true, "12.10": true, "12.2-DEV": true,
		"12.1.4": false, "12.1": false, "11.4.2": false, "13.0": false, "twelve": false, "": false,
	} {
		if got := admitted(version); got != want {
			t.Errorf("admitted(%q) = %v, want %v", version, got, want)
		}
	}
}

func TestTheJDKMajorComesFromItsReleaseFile(t *testing.T) {
	for version, want := range map[string]int{"25.0.5.1": 25, "21": 21, "1.8.0_392": 8} {
		if got, err := readJDK(fakeJDK(t, version)); err != nil || got != want {
			t.Errorf("JAVA_VERSION %q: %d, %v; want %d", version, got, err, want)
		}
	}
	if _, err := readJDK(t.TempDir()); err == nil {
		t.Error("want an error for a directory without a release file")
	}
	empty := t.TempDir()
	os.WriteFile(filepath.Join(empty, "release"), []byte("OS_NAME=\"Linux\"\n"), 0o644)
	if _, err := readJDK(empty); err == nil {
		t.Error("want an error for a release file without JAVA_VERSION")
	}
}

func TestAGhidraAndAJDKREACannotRunTogetherAreNamed(t *testing.T) {
	sys, _ := testSystem(t)
	g122, g1214, gNoFloor := fakeGhidra(t, "12.2", "25"), fakeGhidra(t, "12.1.4", "21"), fakeGhidra(t, "12.3", "")
	jdk21, jdk25 := fakeJDK(t, "21.0.12"), fakeJDK(t, "25.0.5.1")
	noVersion := fakeGhidra(t, "", "25")
	badFloor := fakeGhidra(t, "12.2", "twenty-five")
	for _, c := range []struct {
		name string
		env  map[string]string
		want []string // nil: compatible
	}{
		{"no Ghidra named", map[string]string{"JAVA_HOME": jdk21}, nil},
		{"12.2 on JDK 25", map[string]string{"GHIDRA_INSTALL_DIR": g122, "JAVA_HOME": jdk25}, nil},
		{"12.2 with no JDK named", map[string]string{"GHIDRA_INSTALL_DIR": g122}, nil},
		{"no JDK floor declared", map[string]string{"GHIDRA_INSTALL_DIR": gNoFloor, "JAVA_HOME": jdk21}, nil},
		{"12.1.4", map[string]string{"GHIDRA_INSTALL_DIR": g1214, "JAVA_HOME": jdk21}, []string{`Ghidra "12.1.4"`, "12.2", REAVersion}},
		{"12.2 on JDK 21", map[string]string{"GHIDRA_INSTALL_DIR": g122, "JAVA_HOME": jdk21}, []string{"JDK 21", "JDK 25", `Ghidra "12.2"`}},
		{"not a Ghidra", map[string]string{"GHIDRA_INSTALL_DIR": t.TempDir()}, []string{"not a Ghidra installation"}},
		{"no version", map[string]string{"GHIDRA_INSTALL_DIR": noVersion}, []string{"no Ghidra version"}},
		{"floor not a number", map[string]string{"GHIDRA_INSTALL_DIR": badFloor}, []string{"not a number"}},
		{"JAVA_HOME not a JDK", map[string]string{"GHIDRA_INSTALL_DIR": g122, "JAVA_HOME": t.TempDir()}, []string{"not a JDK"}},
	} {
		err := engineMismatch(sys, c.env)
		if c.want == nil {
			if err != nil {
				t.Errorf("%s: %v", c.name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: want an error", c.name)
			continue
		}
		for _, w := range c.want {
			if !strings.Contains(err.Error(), w) {
				t.Errorf("%s: %q does not name %q", c.name, err, w)
			}
		}
	}
}

func TestAWindowsVariableIsFoundWhateverItsCase(t *testing.T) {
	sys, _ := testSystem(t)
	sys.Platform.OS = "windows"
	env := map[string]string{"Ghidra_Install_Dir": fakeGhidra(t, "12.1.4", "21")}
	if err := engineMismatch(sys, env); err == nil {
		t.Error("want the mixed-case variable to be read")
	}
}

func TestSetupRefusesAnEngineREACannotRunAndSavesNothing(t *testing.T) {
	g122, g1214 := fakeGhidra(t, "12.2", "25"), fakeGhidra(t, "12.1.4", "21")
	jdk21, jdk25 := fakeJDK(t, "21.0.12"), fakeJDK(t, "25.0.5.1")
	for _, c := range []struct {
		name    string
		environ []string
		args    []string
		want    string
	}{
		{"an old Ghidra", nil, []string{"--ghidra", g1214}, `Ghidra "12.1.4"`},
		{"an old JDK given", nil, []string{"--ghidra", g122, "--java-home", jdk21}, "JDK 21"},
		{"an old JDK in the environment", []string{"JAVA_HOME=" + jdk21}, []string{"--ghidra", g122}, "JDK 21"},
		{"a JDK too old for the Ghidra in the environment", []string{"GHIDRA_INSTALL_DIR=" + g122}, []string{"--java-home", jdk21}, "JDK 21"},
	} {
		sys, io := testSystem(t, append(c.environ, "NANO_GPT_API_KEY=k")...)
		fakeToolchain(t, &sys)
		withOpenCode(t, &sys, 0, `{"healthy":true}`)
		code := Main(append([]string{"setup"}, c.args...), sys)
		out := io.stdout.String()
		if code != 1 || !strings.Contains(out, "FAIL  engine") || !strings.Contains(out, c.want) || !strings.Contains(out, "nothing was saved") || !strings.Contains(out, "--java-home") {
			t.Errorf("%s: exit %d:\n%s", c.name, code, out)
		}
		if settings, _ := loadEngineSettings(sys); len(settings) != 0 {
			t.Errorf("%s: saved %v", c.name, settings)
		}
	}

	sys, io := testSystem(t, "NANO_GPT_API_KEY=k", "JAVA_HOME="+jdk21, "GHIDRA_INSTALL_DIR="+g1214)
	fakeToolchain(t, &sys)
	withOpenCode(t, &sys, 0, `{"healthy":true}`)
	if code := Main([]string{"setup", "--ghidra", g122, "--java-home", jdk25}, sys); code != 0 {
		t.Fatalf("a compatible pair over an incompatible environment: exit %d:\n%s", code, io.stdout)
	}
	if settings, _ := loadEngineSettings(sys); settings["GHIDRA_INSTALL_DIR"] != g122 || settings["JAVA_HOME"] != jdk25 {
		t.Errorf("saved %v", settings)
	}
}

func TestALaunchWarnsWhenREAsEngineCannotStart(t *testing.T) {
	g1214, g122, jdk25 := fakeGhidra(t, "12.1.4", "21"), fakeGhidra(t, "12.2", "25"), fakeJDK(t, "25.0.5.1")
	sys, io := testSystem(t, "NANO_GPT_API_KEY=k", "GHIDRA_INSTALL_DIR="+g1214)
	fakeToolchain(t, &sys)
	run := withOpenCode(t, &sys, 0, "")
	if code := Main(nil, sys); code != 0 || len(run.args) == 0 {
		t.Fatalf("exit %d, args %q", code, run.args)
	}
	if err := io.stderr.String(); !strings.Contains(err, "will not start") || !strings.Contains(err, `Ghidra "12.1.4"`) || !strings.Contains(err, "--java-home") {
		t.Errorf("stderr:\n%s", err)
	}

	io.stderr.Reset()
	saveEngineSettings(sys, map[string]string{"GHIDRA_INSTALL_DIR": g122, "JAVA_HOME": jdk25})
	Main(nil, sys)
	if strings.Contains(io.stderr.String(), "will not start") {
		t.Errorf("a compatible saved engine still warns:\n%s", io.stderr)
	}
}

func TestOnlyTheEngineVariablesOfTheSavedSettingsReachREA(t *testing.T) {
	sys, _ := testSystem(t)
	saveEngineSettings(sys, map[string]string{"JAVA_HOME": "/jdk", "REA_LOG_LEVEL": "debug", "NODE_OPTIONS": "--require /x.js", "PATH": "/evil"})
	env, _ := reaEnvironment(sys)
	if len(env) != 2 || env["JAVA_HOME"] != "/jdk" || env["REA_LOG_LEVEL"] != "debug" {
		t.Errorf("env = %v", env)
	}
}

func TestAMetadataFileThatIsNotARegularFileIsRefused(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, "release"), 0o755)
	if _, err := readJDK(dir); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Errorf("err = %v", err)
	}
}
