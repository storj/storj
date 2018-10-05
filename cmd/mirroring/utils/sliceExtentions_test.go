package utils

import (
	"fmt"
	"testing"

	minio "github.com/minio/minio/cmd"
)

//TODO: implement full test
func TestListBucketsWithDifference(t *testing.T) {
	var mainSlice []minio.BucketInfo
	var mirrorSlice []minio.BucketInfo

	mainSlice = append(mainSlice, minio.BucketInfo{Name: "1"})
	mainSlice = append(mainSlice, minio.BucketInfo{Name: "2"})
	mainSlice = append(mainSlice, minio.BucketInfo{Name: "3"})

	mirrorSlice = append(mirrorSlice, minio.BucketInfo{Name: "4"})
	mirrorSlice = append(mirrorSlice, minio.BucketInfo{Name: "2"})

	result := ListBucketsWithDifference(mainSlice, mirrorSlice)

	fmt.Println(result)
}

func TestListObjectsWithDifference(t *testing.T) {

	var mainSlice []minio.ObjectInfo
	var mirrorSlice []minio.ObjectInfo

	//common
	mainSlice = append(mainSlice, minio.ObjectInfo{Name: "1", IsDir: true, Size: 1000, ContentType: "type1"})
	mainSlice = append(mainSlice, minio.ObjectInfo{Name: "2", IsDir: true, Size: 2000, ContentType: "type2"})
	mainSlice = append(mainSlice, minio.ObjectInfo{Name: "3", IsDir: true, Size: 3000, ContentType: "type3"})

	//same name
	mainSlice = append(mainSlice, minio.ObjectInfo{Name: "4", IsDir: false, Size: 4000, ContentType: "type4"})
	mainSlice = append(mainSlice, minio.ObjectInfo{Name: "5", IsDir: false, Size: 5000, ContentType: "type5"})
	mainSlice = append(mainSlice, minio.ObjectInfo{Name: "6", IsDir: false, Size: 6000, ContentType: "type6"})

	//only here
	mainSlice = append(mainSlice, minio.ObjectInfo{Name: "7", IsDir: true, Size: 7000, ContentType: "type7"})
	mainSlice = append(mainSlice, minio.ObjectInfo{Name: "8", IsDir: true, Size: 8000, ContentType: "type8"})

	//common
	mirrorSlice = append(mirrorSlice, minio.ObjectInfo{Name: "1", IsDir: true, Size: 1000, ContentType: "type1"})
	mirrorSlice = append(mirrorSlice, minio.ObjectInfo{Name: "2", IsDir: true, Size: 2000, ContentType: "type2"})
	mirrorSlice = append(mirrorSlice, minio.ObjectInfo{Name: "3", IsDir: true, Size: 3000, ContentType: "type3"})

	//same name
	mirrorSlice = append(mirrorSlice, minio.ObjectInfo{Name: "4", IsDir: false, Size: 4020, ContentType: "type4"})
	mirrorSlice = append(mirrorSlice, minio.ObjectInfo{Name: "5", IsDir: false, Size: 6000, ContentType: "type55"})
	mirrorSlice = append(mirrorSlice, minio.ObjectInfo{Name: "6", IsDir: true, Size: 7000, ContentType: "type6666"})

	//only here
	mirrorSlice = append(mirrorSlice, minio.ObjectInfo{Name: "9", IsDir: true, Size: 7000, ContentType: "type7"})
	mirrorSlice = append(mirrorSlice, minio.ObjectInfo{Name: "10", IsDir: true, Size: 8000, ContentType: "type8"})

	result := ListObjectsWithDifference(mainSlice, mirrorSlice)

	fmt.Println(result)
}

func TestCombineBucketsDistinct(t *testing.T) {
	var mainSlice []minio.BucketInfo
	var mirrorSlice []minio.BucketInfo

	mainSlice = append(mainSlice, minio.BucketInfo{Name: "1"})
	mainSlice = append(mainSlice, minio.BucketInfo{Name: "2"})
	mainSlice = append(mainSlice, minio.BucketInfo{Name: "3"})

	mirrorSlice = append(mirrorSlice, minio.BucketInfo{Name: "1"})
	mirrorSlice = append(mirrorSlice, minio.BucketInfo{Name: "4"})
	mirrorSlice = append(mirrorSlice, minio.BucketInfo{Name: "2"})

	result := CombineBucketsDistinct(mainSlice, mirrorSlice)

	fmt.Println(result)
}
