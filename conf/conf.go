package conf

//Config is used as a json protocol
type Config struct {
	Repository *string // remote address for your repository
	Vcs        *string // must be set in git or hg.
	Android    *AndroidConfig
	Xcode      *XcodeConfig
	Unity      *UnityConfig
}

//AndroidConfig contains some fields for Android
type AndroidConfig struct {
	ProjectPath   *string // must be a releative path to the last config or repository. Keep it nil or blank for not changing the work directory
	Store         *string
	StorePassword *string
	Alias         *string
	AliasPassword *string
}

//XcodeConfig contains some fields for Xcode
type XcodeConfig struct {
	ProjectPath *string
	Sign        *string
	Provision   *string
}

type UnityConfig struct {
	ProjectPath      *string
	Redevelopment    *bool
	BundleIdentifier *string

	// No need to set the Android.ProjectPath and Xcode.ProjectPath in type UnityConfig. Go-pac will changed to the generated project automatically.
	Android *AndroidConfig
	Xcode   *XcodeConfig
}
