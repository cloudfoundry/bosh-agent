package disk

func ConvertFromBytesToMb(sizeInBytes uint64) uint64 {
	return sizeInBytes / (1024 * 1024)
}

func ConvertFromMbToBytes(sizeInMb uint64) uint64 {
	return sizeInMb * 1024 * 1024
}

func ConvertFromKbToBytes(sizeInKb uint64) uint64 {
	return sizeInKb * 1024
}
