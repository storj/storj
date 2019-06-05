/* Code generated by cmd/cgo; DO NOT EDIT. */

/* package storj.io/storj/lib/uplink/ext */


#line 1 "cgo-builtin-export-prolog"

#include <stddef.h> /* for ptrdiff_t below */

#ifndef GO_CGO_EXPORT_PROLOGUE_H
#define GO_CGO_EXPORT_PROLOGUE_H

#ifndef GO_CGO_GOSTRING_TYPEDEF
typedef struct { const char *p; ptrdiff_t n; } _GoString_;
#endif

#endif

/* Start of preamble from import "C" comments.  */


#line 6 "apikey.go"

 #include <stdlib.h>
 #ifndef STORJ_HEADERS
   #define STORJ_HEADERS
   #include "c/headers/main.h"
 #endif

#line 1 "cgo-generated-wrapper"

#line 6 "bucket.go"

 #ifndef STORJ_HEADERS
   #define STORJ_HEADERS
   #include "c/headers/main.h"
 #endif

#line 1 "cgo-generated-wrapper"

#line 6 "common.go"

 #include <stdlib.h>
 #ifndef STORJ_HEADERS
   #define STORJ_HEADERS
   #include "c/headers/main.h"
 #endif

#line 1 "cgo-generated-wrapper"

#line 6 "object.go"

 #ifndef STORJ_HEADERS
   #define STORJ_HEADERS
   #include "c/headers/main.h"
 #endif

#line 1 "cgo-generated-wrapper"

#line 6 "project.go"

 #ifndef STORJ_HEADERS
   #define STORJ_HEADERS
   #include "c/headers/main.h"
 #endif

#line 1 "cgo-generated-wrapper"

#line 6 "uplink.go"

 #ifndef STORJ_HEADERS
   #define STORJ_HEADERS
   #include "c/headers/main.h"
 #endif

#line 1 "cgo-generated-wrapper"


/* End of preamble from import "C" comments.  */


/* Start of boilerplate cgo prologue.  */
#line 1 "cgo-gcc-export-header-prolog"

#ifndef GO_CGO_PROLOGUE_H
#define GO_CGO_PROLOGUE_H

typedef signed char GoInt8;
typedef unsigned char GoUint8;
typedef short GoInt16;
typedef unsigned short GoUint16;
typedef int GoInt32;
typedef unsigned int GoUint32;
typedef long long GoInt64;
typedef unsigned long long GoUint64;
typedef GoInt64 GoInt;
typedef GoUint64 GoUint;
typedef __SIZE_TYPE__ GoUintptr;
typedef float GoFloat32;
typedef double GoFloat64;
typedef float _Complex GoComplex64;
typedef double _Complex GoComplex128;

/*
  static assertion to make sure the file is being used on architecture
  at least with matching size of GoInt.
*/
typedef char _check_for_64_bit_pointer_matching_GoInt[sizeof(void*)==64/8 ? 1:-1];

#ifndef GO_CGO_GOSTRING_TYPEDEF
typedef _GoString_ GoString;
#endif
typedef void *GoMap;
typedef void *GoChan;
typedef struct { void *t; void *v; } GoInterface;
typedef struct { void *data; GoInt len; GoInt cap; } GoSlice;

#endif

/* End of boilerplate cgo prologue.  */

#ifdef __cplusplus
extern "C" {
#endif


// ParseAPIKey parses an API Key

extern APIKeyRef_t ParseAPIKey(char* p0, char** p1);

// Serialize serializes the API Key to a string

extern char* Serialize(APIKeyRef_t p0);

extern ObjectRef_t OpenObject(BucketRef_t p0, char* p1, char** p2);

extern void UploadObject(BucketRef_t p0, char* p1, BufferRef_t p2, UploadOptions_t* p3, char** p4);

extern ObjectList_t ListObjects(BucketRef_t p0, ListOptions_t* p1, char** p2);

extern void CloseBucket(BucketRef_t p0, char** p1);

extern IDVersion_t GetIDVersion(unsigned int p0, char** p1);

extern BufferRef_t NewBuffer();

extern void WriteBuffer(BufferRef_t p0, Bytes_t* p1, char** p2);

extern void ReadBuffer(BufferRef_t p0, Bytes_t* p1, char** p2);

extern void CloseObject(ObjectRef_t p0, char** p1);

extern DownloadReaderRef_t DownloadRange(ObjectRef_t p0, int64_t p1, int64_t p2, char** p3);

extern int Download(DownloadReaderRef_t p0, Bytes_t* p1, char** p2);

extern ObjectMeta_t ObjectMeta(ObjectRef_t p0, char** p1);

extern Bucket_t CreateBucket(ProjectRef_t p0, char* p1, BucketConfig_t* p2, char** p3);

extern BucketRef_t OpenBucket(ProjectRef_t p0, char* p1, EncryptionAccess_t* p2, char** p3);

extern void DeleteBucket(ProjectRef_t p0, char* p1, char** p2);

extern BucketList_t ListBuckets(ProjectRef_t p0, BucketListOptions_t* p1, char** p2);

extern BucketInfo_t GetBucketInfo(ProjectRef_t p0, char* p1, char** p2);

extern void CloseProject(ProjectRef_t p0, char** p1);

extern UplinkRef_t NewUplink(char** p0);

extern UplinkRef_t NewUplinkInsecure(char** p0);

extern ProjectRef_t OpenProject(UplinkRef_t p0, char* p1, APIKeyRef_t p2, char** p3);

extern void CloseUplink(UplinkRef_t p0, char** p1);

#ifdef __cplusplus
}
#endif
