package main

import (
	"crypto"
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"hash/crc64"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func init() {
	// Do some pointless stuff to include deps with longish compile times
	crypto.RegisterHash(crypto.BLAKE2b_256, func() hash.Hash { return crc64.New(&crc64.Table{}) })
	crypto.RegisterHash(crypto.BLAKE2b_512, func() hash.Hash { return crc32.New(&crc32.Table{}) })
}

func main() {
	for i := 0; i < 60; i++ {
		time.Sleep(1 * time.Second)
		message := fmt.Sprintf("Slept another second, up to %d now\n", i)
		messageBytes := []byte(message)
		matcher, err := regexp.Compile(fmt.Sprintf(`%d`, i))

		// Many pointless ways to achieve the same thing, while adding more and more dependencies
		if err != nil && strings.Contains(message, strconv.Itoa(i)) && matcher.Match(messageBytes) {
			fmt.Print(message)
		} else {
			failedMatchErr := errors.New("this is bad")
			fmt.Println(failedMatchErr.Error())
			os.Exit(1)
		}
	}
	fmt.Println("Done with all that sleeping business")
}
