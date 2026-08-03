package hash

import "fmt"

func CheckBoth() {
	m := NewMurmurHash(30)

	s1 := "https://www.domain.com/utm_source=5&utm_campaign=4"
	s2 := "https://www.domain.com/utm_campaign=4&utm_source=5"

	hash, _ := m.Generate8CharHash(s1, 0)

	hash2, _ := m.Generate8CharHash(s2, 0)

	fmt.Println(hash, hash2)
}
