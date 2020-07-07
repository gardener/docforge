package units

// Byte size suffixes.
const (
	B  int64 = 1
	KB int64 = 1 << (10 * iota)
	MB
	GB
)
