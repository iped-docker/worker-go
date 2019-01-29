package main

import (
	"io"
	"os"
)

func main() {
	out, _ := os.Create("swagger.go")
	out.Write([]byte("package main \n\nconst (\n"))
	out.Write([]byte("generatedSwagger = `"))
	f, _ := os.Open("generate/swagger.json")
	io.Copy(out, f)
	out.Write([]byte("`\n"))
	out.Write([]byte(")\n"))
}
