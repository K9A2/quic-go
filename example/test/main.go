package main

import "fmt"

func main() {
	array := []int{1, 2, 3, 4, 5}
	for _, value := range array {
		fmt.Print(value, " ")
	}
	fmt.Print("\n")
	delete()
}
