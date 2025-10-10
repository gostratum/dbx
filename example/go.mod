module github.com/gostratum/dbx/example

go 1.25

require (
	github.com/gin-gonic/gin v1.10.0
	github.com/gostratum/core v0.1.0
	github.com/gostratum/dbx v0.1.0
	github.com/gostratum/httpx v0.1.0
	go.uber.org/fx v1.22.2
	gorm.io/gorm v1.25.12
)

replace github.com/gostratum/dbx => ../