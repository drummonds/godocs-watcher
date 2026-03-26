module codeberg.org/hum3/godocs-watcher

go 1.25.3

require (
	codeberg.org/hum3/godocs-client v0.1.0
	github.com/fsnotify/fsnotify v1.9.0
	gopkg.in/yaml.v3 v3.0.1
)

require golang.org/x/sys v0.13.0 // indirect

replace codeberg.org/hum3/godocs-client => ../godocs-client
