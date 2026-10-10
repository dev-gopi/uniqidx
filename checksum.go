package uniqidx

var verhoeffD = [10][10]uint8{{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, {1, 2, 3, 4, 0, 6, 7, 8, 9, 5}, {2, 3, 4, 0, 1, 7, 8, 9, 5, 6}, {3, 4, 0, 1, 2, 8, 9, 5, 6, 7}, {4, 0, 1, 2, 3, 9, 5, 6, 7, 8}, {5, 9, 8, 7, 6, 0, 4, 3, 2, 1}, {6, 5, 9, 8, 7, 1, 0, 4, 3, 2}, {7, 6, 5, 9, 8, 2, 1, 0, 4, 3}, {8, 7, 6, 5, 9, 3, 2, 1, 0, 4}, {9, 8, 7, 6, 5, 4, 3, 2, 1, 0}}
var verhoeffP = [8][10]uint8{{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, {1, 5, 7, 6, 2, 8, 3, 0, 9, 4}, {5, 8, 0, 3, 7, 9, 6, 1, 4, 2}, {8, 9, 1, 6, 0, 4, 3, 5, 2, 7}, {9, 4, 5, 3, 1, 2, 6, 8, 7, 0}, {4, 2, 8, 6, 5, 7, 3, 9, 0, 1}, {2, 7, 9, 3, 8, 0, 6, 4, 1, 5}, {7, 0, 4, 6, 9, 1, 3, 2, 5, 8}}
var verhoeffInv = [10]uint8{0, 4, 3, 2, 1, 5, 6, 7, 8, 9}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range s {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
func VerhoeffDigit(s string) byte {
	if !allDigits(s) {
		panic("uniqidx: VerhoeffDigit requires decimal digits")
	}
	var c uint8
	for i, j := 0, len(s)-1; j >= 0; i, j = i+1, j-1 {
		c = verhoeffD[c][verhoeffP[(i+1)%8][s[j]-'0']]
	}
	return '0' + verhoeffInv[c]
}
func VerhoeffValid(s string) bool {
	if !allDigits(s) {
		return false
	}
	var c uint8
	for i, j := 0, len(s)-1; j >= 0; i, j = i+1, j-1 {
		c = verhoeffD[c][verhoeffP[i%8][s[j]-'0']]
	}
	return c == 0
}
func LuhnDigit(s string) byte {
	if !allDigits(s) {
		panic("uniqidx: LuhnDigit requires decimal digits")
	}
	sum := 0
	dbl := true
	for i := len(s) - 1; i >= 0; i-- {
		d := int(s[i] - '0')
		if dbl {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		dbl = !dbl
	}
	return byte('0' + (10-sum%10)%10)
}
func LuhnValid(s string) bool {
	if !allDigits(s) {
		return false
	}
	sum := 0
	dbl := false
	for i := len(s) - 1; i >= 0; i-- {
		d := int(s[i] - '0')
		if dbl {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		dbl = !dbl
	}
	return sum%10 == 0
}
