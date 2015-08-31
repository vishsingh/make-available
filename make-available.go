package main

/*
runit() {
  go install github.com/vishsingh/make-available && /home/work/bin/make-available
}
*/

import "fmt"
import "os/exec"

func withEcho(thunk func()) {
	fmt.Printf("starting\n")
	defer fmt.Printf("ending\n")

	thunk()
}

func main() {
	fmt.Printf("hello\n")

	c := exec.Command("/bin/ls")

	output, err := c.Output()

	if err != nil {
		fmt.Printf("error was %s\n", err.Error())
	} else {
		fmt.Printf("no error\n")
		fmt.Printf("output was %q\n", output)
	}

	fmt.Printf("\n")

	withEcho(func() {
		fmt.Printf("middle\n")

		withEcho(func() {
			fmt.Printf("inside\n")
			panic("OOPS")
		})
	})
}
