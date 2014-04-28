package worker

import "path/filepath"
import "fmt"
import "strings"
import "os"
import "io/ioutil"
import "encoding/json"
import "github.com/GarX/go-pac/logger"
import "github.com/GarX/go-pac/conf"
import "github.com/GarX/go-pac/cmd"
import "errors"
import "os/user"
import "regexp"

var antPropertyTemplate string
var unityBuildTemplate0, unityBuildTemplate1, unityBuildTemplate2 string

var (
	workdir, _ = os.Getwd() //record the base info
	home       string
)

func init() {
	antPropertyTemplate = "key.store=%s\n" +
		"key.alias=%s\n" +
		"key.store.password=%s\n" +
		"key.alias.password=%s"
	unityBuildTemplate0 = "using UnityEngine;\n" +
		"using UnityEditor;\n" +
		"public class GoPacBuild\n" +
		"{\n" +
		"public static void main()\n" +
		"{\n" +
		"PlayerSettings.bundleIdentifier = \"%s\";\n" +
		"PlayerSettings.iPhoneBundleIdentifier= \"%s\";\n"
	unityBuildTemplate1 = "PlayerSettings.Android.keystoreName=\"%s\";\n" +
		"PlayerSettings.Android.keystorePass=\"%s\";\n" +
		"PlayerSettings.Android.keyaliasName=\"%s\";\n" +
		"PlayerSettings.Android.keyaliasPass=\"%s\";\n"
	unityBuildTemplate2 = "BuildOptions opt = BuildOptions.SymlinkLibraries | BuildOptions.Development | BuildOptions.ConnectWithProfiler | BuildOptions.AllowDebugging;\n" +
		"string[] scenes = {%s};\n" +
		"BuildPipeline.BuildPlayer(scenes,\"./GoPacPrj\",BuildTarget.%s,opt);\n" +
		"}\n" +
		"}\n"
	u, _ := user.Current()
	home = u.HomeDir
}

//Run it.
func Run(filename, outfile string) (err error) {
	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer file.Close()
	b, err := ioutil.ReadAll(file)
	if err != nil {
		return
	}
	config := new(conf.Config)
	err = json.Unmarshal(b, config)
	if err != nil {
		return
	}
	if config.Repository == nil {
		err = errors.New("repository must be set")
		return
	}
	err = fetchFromRemote(*config.Repository)
	if err != nil {
		return
	}
	if config.Android != nil {
		return compileAndroid(config.Android, outfile)
	}
	if config.Xcode != nil {
		return compileXcode(config.Xcode, outfile)
	}
	if config.Unity != nil {
		return compileUnity(config.Unity, outfile)
	}
	return
}

func compileUnity(config *conf.UnityConfig, outfile string) (err error) {
	// find the *.unity file under the ./Assets recursively.

	assetsDir, err := os.Open("./Assets")
	defer assetsDir.Close()
	if err != nil {
		return
	}
	unityReg, err := regexp.Compile("\\.unity$")
	if err != nil {
		return
	}
	curwd, err := os.Getwd()
	if err != nil {
		return
	}
	_, files := findRecursively(assetsDir, unityReg, curwd+"/Assets")
	if len(files) == 0 {
		err = errors.New("no .unity file was found")
		return
	}
	// make up a scenes string to generate the build file
	// scenes string like like such pattern: "a.unity","b.unity","c,unity"
	scenes := strings.Join(files, "\",\"")
	scenes = "\"" + scenes + "\""
	logger.Debug("Found scenes: " + scenes)
	var buildTarget string
	if config.Android != nil {
		buildTarget = "Android"
	}
	if config.Xcode != nil {
		buildTarget = "iPhone"
	}
	if buildTarget == "" {
		err = errors.New("no unity target specified")
		return
	}
	// get bundleIdentifier,if it is not set.just keep it as a empty string.
	var bundle string
	if config.BundleIdentifier != nil {
		bundle = *config.BundleIdentifier
	}
	var content, content0, content1, content2 string
	content0 = fmt.Sprintf(unityBuildTemplate0, bundle, bundle)
	content2 = fmt.Sprintf(unityBuildTemplate2, scenes, buildTarget)
	if androidCanSign(config.Android) == true {
		content1 = fmt.Sprintf(unityBuildTemplate1, *config.Android.Store, *config.Android.StorePassword, *config.Android.Alias, *config.Android.AliasPassword)
	} else {
		content1 = fmt.Sprintf(unityBuildTemplate1, "", "", "", "")
	}
	content = content0 + content1 + content2
	err = os.MkdirAll("./Assets/Editor", 0755)
	if err != nil {
		return
	}
	os.Remove("./Assets/Editor/GoPacBuild.cs")
	csFile, err := os.OpenFile("./Assets/Editor/GoPacBuild.cs", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return
	}
	csFile.WriteString(content)
	csFile.Close()

	// start unity project building
	os.Remove(home + "/Library/Logs/Unity/Editor.log")
	// store the error and not return instantly
	// return the error after reading the log
	errtmp := cmd.SyncCmd("/Applications/Unity/Unity.app/Contents/MacOS/Unity", []string{"-batchMode", "-projectPath", curwd, "-executeMethod", "GoPacBuild.main", "-quit"})

	//read the log file
	unityLog, err := os.Open(home + "/Library/Logs/Unity/Editor.log")
	defer unityLog.Close()
	if err != nil {
		return
	}
	bytes, err := ioutil.ReadAll(unityLog)
	if err != nil {
		return
	}
	logger.Debug(string(bytes))
	if errtmp != nil {
		err = errtmp
		return
	}
	// check redevelopment settings. set it false if it is not set in the json file
	if config.Redevelopment == nil {
		config.Redevelopment = new(bool)
		*config.Redevelopment = false
	}
	if config.Xcode != nil {
		if *config.Redevelopment == true {
			err = cmd.SyncCmd("mv", []string{"-v", "GoPacPrj", filepath.Base(outfile)}) //rename the project
			if err != nil {
				return
			}
			if strings.HasPrefix(outfile, "/") == false { // change the file to absolute path
				outfile = workdir + "/" + outfile
			}
			err = cmd.SyncCmd("mv", []string{"-v", filepath.Base(outfile), filepath.Dir(outfile)}) // move the directory to the specified path
			if err != nil {
				return
			}
		} else {
			err = os.Chdir("./GoPacPrj")
			err = compileXcode(config.Xcode, outfile)
		}
		return
	}
	if config.Android != nil {
		if strings.HasPrefix(outfile, "/") == false {
			outfile = workdir + "/" + outfile
		}
		if *config.Redevelopment == true {
			err = os.MkdirAll(outfile, 0777)
			if err != nil {
				return
			}
			err = cmd.SyncCmd("unzip", []string{"GoPacPrj", "-d", outfile})
			if err != nil {
				return
			}
		} else {
			err = cmd.SyncCmd("cp", []string{"GoPacPrj", outfile})
			if err != nil {
				return
			}
		}
		return
	}
	return
}

func AbPath(p string) string {
	if strings.HasPrefix(p, "/") == false {
		return workdir + "/" + p
	} else {
		return p
	}
}

func androidCanSign(config *conf.AndroidConfig) bool {
	if config == nil {
		return false
	}
	if config.Store == nil || config.StorePassword == nil || config.Alias == nil || config.AliasPassword == nil {
		return false
	} else {
		return true
	}
}

// find the directories and files which matches the regular expression
func findRecursively(dir *os.File, reg *regexp.Regexp, currentwd string) (dirs, files []string) {
	dirs = make([]string, 0)
	files = make([]string, 0)

	fileInfos, err := dir.Readdir(0)
	if err != nil {
		return
	}
	for i := 0; i < len(fileInfos); i++ {
		name := fileInfos[i].Name()
		if reg.Match([]byte(name)) == true {
			if fileInfos[i].IsDir() == true {
				dirs = append(dirs, currentwd+"/"+name)
			} else {
				files = append(dirs, currentwd+"/"+name)
			}
		}
		if fileInfos[i].IsDir() == true {
			redir, err := os.Open(currentwd + "/" + name)
			if err != nil {
				continue
			}
			newDirs, newFiles := findRecursively(redir, reg, currentwd+"/"+name)
			dirs = append(dirs, newDirs...)
			files = append(files, newFiles...)
			redir.Close()
		}
	}
	return
}

func compileXcode(config *conf.XcodeConfig, outfile string) (err error) {
	// find the *.xcodeproj file under the working directory
	// if multiple xcodeproj file exists, it picks one of them.
	var wdnow string
	wdnow, err = os.Getwd()
	if err != nil {
		return
	}
	wdf, err := os.Open(wdnow)
	defer wdf.Close()
	if err != nil {
		return
	}
	logger.Debug("Read Directory " + wdnow)
	fileInfos, err := wdf.Readdir(0)

	if err != nil {
		return
	}
	var prjName string //project name without *.xcodeproj suffix
	for i := 0; i < len(fileInfos); i++ {
		if fileInfos[i].IsDir() == false { // .xcodeproj file is a directory
			continue
		}
		if strings.HasSuffix(fileInfos[i].Name(), ".xcodeproj") == true {
			bytes := []byte(fileInfos[i].Name())
			prjName = string(bytes[:len(bytes)-10])
			break
		}
	}
	if prjName == "" {
		err = errors.New("cannot find the *.xcodeproj file")
		return
	}
	logger.Debug("Found target " + prjName)

	// clean the project
	err = cmd.SyncCmd("xcodebuild", []string{"-sdk", "iphoneos", "-target", prjName, "-configuration", "Release", "clean"})
	if err != nil {
		return
	}

	// build the project. If config.Sign is set, use the Sign.
	if config.Sign == nil {
		err = cmd.SyncCmd("xcodebuild", []string{"-sdk", "iphoneos", "-target", prjName, "-configuration", "Release", "CODE_SIGN_IDENTITY=", "CODE_SIGNING_REQUIRED=NO"})
	} else {
		err = cmd.SyncCmd("xcodebuild", []string{"-sdk", "iphoneos", "-target", prjName, "-configuration", "Release", "CODE_SIGN_IDENTITY=\"" + *config.Sign + "\""})
	}
	if err != nil {
		logger.Debug("xcodebuild failed")
		return
	}
	// find the .app file,mostly in ./build or ./build/Release-iphoneos
	var appPath string
	buildDir, err := os.Open("./build")
	if err != nil {
		return
	}
	defer buildDir.Close()
	fileInfos, err = buildDir.Readdir(0)
	if err != nil {
		return
	}
	buildwd, err := os.Getwd()
	if err != nil {
		return
	}
	for i := 0; i < len(fileInfos); i++ {
		// .app file is a dir
		if fileInfos[i].IsDir() == false {
			continue
		}
		if strings.HasSuffix(fileInfos[i].Name(), ".app") == true {
			appPath = buildwd + "/build/" + fileInfos[i].Name()
			break
		}
	}
	if appPath == "" { //Not found in ./build,try to find it in ./build/Release-iphoneos
		var releaseDir *os.File
		releaseDir, err = os.Open("./build/Release-iphoneos")
		defer releaseDir.Close()
		if err == nil {
			fileInfos, err = releaseDir.Readdir(0)
			if err != nil {
				return
			}
			for i := 0; i < len(fileInfos); i++ {
				if fileInfos[i].IsDir() == false {
					continue
				}
				if strings.HasSuffix(fileInfos[i].Name(), ".app") == true {
					appPath = buildwd + "/build/Release-iphoneos/" + fileInfos[i].Name()
					break
				}
			}
		}
	}
	if appPath == "" {
		err = errors.New(".app file not found")
		return
	}
	logger.Debug("Find .app file at " + appPath)
	// pack the .app into .ipa
	// change the working directory before run xcrun
	// appPath is a absolute path and will not be affected by chdir
	// outfile here must be a absolute path.
	logger.Debug("Enter " + workdir)
	os.Chdir(workdir)
	if strings.HasPrefix(outfile, "/") == false {
		outfile = workdir + "/" + outfile
	}
	if config.Provision == nil {
		logger.Debug("Provision not found!")
		logger.Debug("Make ipa at " + outfile)
		err = cmd.SyncCmd("xcrun", []string{"-sdk", "iphoneos", "PackageApplication", appPath, "-o", outfile})
	} else {
		err = cmd.SyncCmd("xcrun", []string{"-sdk", "iphoneos", "PackageApplication", appPath, "-o", outfile, "--embed", *config.Provision})
	}
	return
}

func compileAndroid(config *conf.AndroidConfig, outfile string) (err error) {
	var file *os.File

	// remove the old build
	err = cmd.SyncCmd("ant", []string{"clean", "-Dsdk.dir=/usr/lib/android/sdk"})
	if err != nil {
		return
	}
	sign := androidCanSign(config)
	// generate ant.properties for ant to sign the apk while compliling and packing.
	if sign == true {
		str := fmt.Sprintf(antPropertyTemplate, AbPath(*config.Store), *config.StorePassword, *config.Alias, *config.AliasPassword)
		file, err = os.OpenFile("ant.properties", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0777)
		if err != nil {
			return
		}
		_, err = file.WriteString(str)
		if err != nil {
			return
		}
	}
	// use ant release to build and pack the project.
	err = cmd.SyncCmd("ant", []string{"release", "-Dsdk.dir=/usr/lib/android/sdk"})
	if err != nil {
		return
	}

	// remove the ant.properties will avoid the git conflict
	if sign == true {
		err = os.Remove("ant.properties")
		if err != nil {
			return
		}
	}

	// Anaylse the build directory and found .apk file.
	// If sign is set false. find -unsigned.apk
	// if sign is set true. find the .apk without -unsigned and -unaligned
	buildDir, err := os.Open("./bin")
	if err != nil {
		return err
	}
	fileInfos, err := buildDir.Readdir(0)
	if err != nil {
		return err
	}

	var targetApkPath string
	for i := 0; i < len(fileInfos); i++ {
		if fileInfos[i].IsDir() == true {
			continue
		}
		filename := fileInfos[i].Name()
		if strings.HasSuffix(filename, ".apk") == false || strings.HasSuffix(filename, "-unaligned.apk") == true {
			continue
		}
		if sign == strings.HasSuffix(filename, "-unsigned.apk") {
			continue
		} else {
			var wd string
			wd, err = os.Getwd()
			if err != nil {
				return
			}
			targetApkPath = wd + "/bin/" + filename
			break
		}
	}

	if targetApkPath == "" {
		err = errors.New("no apk found")
	}
	logger.Debug("Find " + targetApkPath)
	// targetApkPath records the absolute path of target apk.
	// change the working dirctory last time and copy the apk file

	logger.Debug("Enter" + workdir)
	err = os.Chdir(workdir) //recover the work directory caused by fetchFromRemote
	if err != nil {
		return
	}
	logger.Debug("copy " + targetApkPath + " to " + outfile)
	cmd.SyncCmd("cp", []string{"-R", targetApkPath, outfile})

	defer func() {
		file.Close()
		buildDir.Close()
	}()
	return
}

// clone or pull the repo from remote
// ATTENTION!!! : The working directory changed after the function is called
func fetchFromRemote(repo string) (err error) {
	logger.Debug("Home: " + home)
	owner, dir, err := getRepoDir(repo)
	if err != nil {
		return
	}
	err = os.MkdirAll(home+"/Library/go-pac/"+owner+"/"+dir, 0755)
	if err != nil {
		return
	}
	logger.Debug("Enter ~/Library/go-pac/" + owner + "/" + dir)
	err = os.Chdir(home + "/Library/go-pac/" + owner + "/" + dir)
	if err != nil {
		return
	}
	err = cmd.SyncCmd("git", []string{"init"})
	if err != nil {
		return
	}
	err = cmd.SyncCmd("git", []string{"pull", repo})
	if err != nil {
		return
	}
	return nil
}

//return the repo's owner and name.
func getRepoDir(repo string) (string, string, error) {
	strs := strings.Split(repo, "/")
	if len(strs) == 1 {
		return "", "", errors.New("invalid repository")
	}
	return strs[len(strs)-2], strs[len(strs)-1], nil
}
