package cachemanager

func getPageCount(itemCount uint64) uint64 {
	if itemCount%PageSize == 0 {
		return itemCount / PageSize
	} else {
		return (itemCount / PageSize) + 1
	}
}
