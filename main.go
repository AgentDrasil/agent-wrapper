package main

import (
 "fmt"
 "log"

 "go.mitchellh.com/libghostty"
)

func main() {
 term, err := libghostty.NewTerminal(libghostty.WithSize(80, 24))
 if err != nil {
  log.Fatal(err)
 }
 defer term.Close()

 // Feed VT data — bold green "world", then plain text.
 fmt.Fprintf(term, "Hello, \033[1;32mworld\033[0m!\r\n")

 // Format the terminal contents as plain text.
 f, err := libghostty.NewFormatter(term,
  libghostty.WithFormatterFormat(libghostty.FormatterFormatPlain),
  libghostty.WithFormatterTrim(true),
 )
 if err != nil {
  log.Fatal(err)
 }
 defer f.Close()

 output, _ := f.FormatString()
 fmt.Println(output) // Hello, world!
}
