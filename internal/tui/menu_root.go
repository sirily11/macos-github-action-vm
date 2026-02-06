package tui

func rootMenuEntries() []menuEntry {
	return []menuEntry{
		setupMenuItem{},
		buildMenuItem{},
		configMenuItem{},
		runMenuItem{},
		imagesMenuItem{},
		daemonMenuItem{},
		viewLogsMenuItem{},
		quitMenuItem{},
	}
}
