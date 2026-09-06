package selfupdate

// Status describes the result of a launch-time update check. It is serialized to
// the dashboard, which renders a loud or quiet notice based on Tier and offers a
// one-click update only when SelfUpdateSupported is true.
type Status struct {
	CurrentVersion      string `json:"current_version"`
	LatestVersion       string `json:"latest_version"`
	LatestReleaseURL    string `json:"latest_release_url"`
	Tier                Tier   `json:"tier"`
	UpdateAvailable     bool   `json:"update_available"`
	SelfUpdateSupported bool   `json:"self_update_supported"`
	Reason              string `json:"reason"`
	PackageManager      string `json:"package_manager"`
	OS                  string `json:"os"`
}
