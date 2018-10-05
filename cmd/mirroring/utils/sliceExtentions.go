package utils

import (
	minio "github.com/minio/minio/cmd"
	"storj.io/mirroring/models"
	"fmt"
)

//ListBucketsWithDifference used to show different and common elements in 2 []minio.BucketInfo slices
func ListBucketsWithDifference(mainBuckets, mirrorBuckets []minio.BucketInfo) (result []models.BucketDiffModel) {
	mainLength := len(mainBuckets)
	mirrorLength := len(mirrorBuckets)
	keys := make(map[string]bool)

	for i := 0; i < 2; i++ {
		for j := 0; j < mainLength; j++ {
			found := false
			temp := mainBuckets[j]

			for k := 0; k < mirrorLength; k++ {

				if temp.Name == mirrorBuckets[k].Name {
					found = true

					if _, value := keys[temp.Name]; !value {
						keys[temp.Name] = true

						diffModel := models.BucketDiffModel{
							Name: temp.Name,
						}

						diffModel.Diff.AddFlag(models.NAME)
						diffModel.Diff.AddFlag(models.IN_MAIN)
						diffModel.Diff.AddFlag(models.IN_MIRROR)

						result = append(result, diffModel)
					}

					break
				}
			}

			// String not found. We add it to return slice
			if !found {
				if _, value := keys[temp.Name]; !value {
					keys[temp.Name] = true

					diffModel := models.BucketDiffModel{
						Name: temp.Name,
					}

					if i == 0 {
						diffModel.Diff.AddFlag(models.IN_MAIN)
					} else {
						diffModel.Diff.AddFlag(models.IN_MIRROR)
					}

					result = append(result, diffModel)
				}
			}
		}

		//Swap the slices, only if it was the first loop
		if i == 0 {
			mainBuckets, mirrorBuckets = mirrorBuckets, mainBuckets
			mainLength = len(mainBuckets)
			mirrorLength = len(mirrorBuckets)
		}
	}

	return result
}

//ListObjectsWithDifference used to show different and common elements in 2 []minio.ListObjectsInfo slices
func ListObjectsWithDifference(mainObjects, mirrorObjects []minio.ObjectInfo) (result []models.ObjectDiffModel) {
	mainLength := len(mainObjects)
	mirrorLength := len(mirrorObjects)
	keys := make(map[string]bool)

	for i := 0; i < 2; i++ {
		for j := 0; j < mainLength; j++ {
			found := false
			mainFile := mainObjects[j]

			for k := 0; k < mirrorLength; k++ {
				mirrorFile := mirrorObjects[k]

				if mainFile.Name == mirrorFile.Name {
					found = true
					if _, value := keys[mainFile.Name]; !value {
						keys[mainFile.Name] = true

						diffModel := models.ObjectDiffModel{
							Name: mainFile.Name,
						}

						diffModel.Diff.AddFlag(models.NAME)
						diffModel.Diff.AddFlag(models.IN_MAIN)
						diffModel.Diff.AddFlag(models.IN_MIRROR)

						if mainFile.Size == mirrorFile.Size {
							diffModel.Diff.AddFlag(models.SIZE)
						}

						if mainFile.ContentType == mirrorFile.ContentType {
							diffModel.Diff.AddFlag(models.CONTENT_TYPE)
						}

						if mainFile.IsDir == mirrorFile.IsDir {
							diffModel.Diff.AddFlag(models.IS_DIR)
						}

						result = append(result, diffModel)
						break
					}

				}
			}

			// String not found. We add it to return slice
			if !found {

				if _, value := keys[mainFile.Name]; !value {
					keys[mainFile.Name] = true

					diffModel := models.ObjectDiffModel{
						Name: mainFile.Name,
					}

					if i == 0 {
						diffModel.Diff.AddFlag(models.IN_MAIN)
					} else {
						diffModel.Diff.AddFlag(models.IN_MIRROR)
					}

					result = append(result, diffModel)
				}
			}
		}

		//Swap the slices, only if it was the first loop
		if i == 0 {
			mainObjects, mirrorObjects = mirrorObjects, mainObjects
			mainLength = len(mainObjects)
			mirrorLength = len(mirrorObjects)
		}
	}

	return result
}

//CombineBucketsDistinct is used to combine two []minio.BucketInfo slices without repeating elements
func CombineBucketsDistinct(mainBuckets []minio.BucketInfo, mirrorBuckets []minio.BucketInfo) (result []minio.BucketInfo) {
	mainLength := len(mainBuckets)
	mirrorLength := len(mirrorBuckets)
	totalLength := mirrorLength + mainLength
	tempSlice := mainBuckets

	keys := make(map[string]bool)

	for i := 0; i < totalLength; i++ {
		iterator := i

		if i >= mainLength {
			tempSlice = mirrorBuckets
			iterator = i - mainLength
		}

		if _, value := keys[tempSlice[iterator].Name]; !value {
			keys[tempSlice[iterator].Name] = true
			result = append(result, tempSlice[iterator])
		}
	}

	return result
}

func CombineObjectsDist(objs []interface{}, objs2 []interface{}) (result []interface{}) {
	switch v:= objs[0].(type) {
		case minio.ObjectInfo:
			fmt.Println(v)
	}

	return result
}

//CombineObjectsDistinct is used to combine two []minio.BucketInfo slices without repeating elements
func CombineObjectsDistinct(mainBuckets []minio.ObjectInfo, mirrorBuckets []minio.ObjectInfo) (result []minio.ObjectInfo) {
	mainLength := len(mainBuckets)
	mirrorLength := len(mirrorBuckets)
	totalLength := mirrorLength + mainLength
	tempSlice := mainBuckets

	keys := make(map[string]bool)

	for i := 0; i < totalLength; i++ {
		iterator := i

		if i >= mainLength {
			tempSlice = mirrorBuckets
			iterator = i - mainLength
		}

		if _, value := keys[tempSlice[iterator].Name]; !value {
			keys[tempSlice[iterator].Name] = true
			result = append(result, tempSlice[iterator])
		}
	}

	return result
}
