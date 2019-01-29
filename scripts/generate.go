package main

import (
	"io"
	"os"
)

// Reads all .txt files in the current folder
// and encodes them as strings literals in textfiles.go
func main() {
	out, _ := os.Create("generated.go")
	out.Write([]byte("package main \n\nconst (\n"))
	out.Write([]byte("swagger_content = `"))
	f, _ := os.Open("static/swagger.json")
	io.Copy(out, f)
	out.Write([]byte("`\n"))
	out.Write([]byte(")\n"))
}
