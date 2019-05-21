/* Generated by the protocol buffer compiler.  DO NOT EDIT! */
/* Generated from: uplink.proto */

#ifndef PROTOBUF_C_uplink_2eproto__INCLUDED
#define PROTOBUF_C_uplink_2eproto__INCLUDED

#include <protobuf-c/protobuf-c.h>

PROTOBUF_C__BEGIN_DECLS

#if PROTOBUF_C_VERSION_NUMBER < 1003000
# error This file was generated by a newer version of protoc-c which is incompatible with your libprotobuf-c headers. Please update your headers.
#elif 1003001 < PROTOBUF_C_MIN_COMPILER_VERSION
# error This file was generated by an older version of protoc-c which is incompatible with your libprotobuf-c headers. Please regenerate this file with a newer version of protoc-c.
#endif


typedef struct _Storj__Libuplink__IDVersion Storj__Libuplink__IDVersion;
typedef struct _Storj__Libuplink__TLSConfig Storj__Libuplink__TLSConfig;
typedef struct _Storj__Libuplink__UplinkConfig Storj__Libuplink__UplinkConfig;
typedef struct _Storj__Libuplink__EncryptionParameters Storj__Libuplink__EncryptionParameters;
typedef struct _Storj__Libuplink__RedundancyScheme Storj__Libuplink__RedundancyScheme;
typedef struct _Storj__Libuplink__BucketConfig Storj__Libuplink__BucketConfig;


/* --- enums --- */


/* --- messages --- */

struct  _Storj__Libuplink__IDVersion
{
  ProtobufCMessage base;
  uint32_t number;
  uint64_t new_private_key;
};
#define STORJ__LIBUPLINK__IDVERSION__INIT \
 { PROTOBUF_C_MESSAGE_INIT (&storj__libuplink__idversion__descriptor) \
    , 0, 0 }


struct  _Storj__Libuplink__TLSConfig
{
  ProtobufCMessage base;
  protobuf_c_boolean skip_peer_ca_whitelist;
  char *peer_ca_whitelist_path;
};
#define STORJ__LIBUPLINK__TLSCONFIG__INIT \
 { PROTOBUF_C_MESSAGE_INIT (&storj__libuplink__tlsconfig__descriptor) \
    , 0, (char *)protobuf_c_empty_string }


struct  _Storj__Libuplink__UplinkConfig
{
  ProtobufCMessage base;
  Storj__Libuplink__TLSConfig *tls;
  Storj__Libuplink__IDVersion *identity_version;
  char *peer_id_version;
  int64_t max_inline_size;
  int64_t max_memory;
};
#define STORJ__LIBUPLINK__UPLINK_CONFIG__INIT \
 { PROTOBUF_C_MESSAGE_INIT (&storj__libuplink__uplink_config__descriptor) \
    , NULL, NULL, (char *)protobuf_c_empty_string, 0, 0 }


struct  _Storj__Libuplink__EncryptionParameters
{
  ProtobufCMessage base;
  ProtobufCBinaryData cipher_suite;
  int32_t block_size;
};
#define STORJ__LIBUPLINK__ENCRYPTION_PARAMETERS__INIT \
 { PROTOBUF_C_MESSAGE_INIT (&storj__libuplink__encryption_parameters__descriptor) \
    , {0,NULL}, 0 }


struct  _Storj__Libuplink__RedundancyScheme
{
  ProtobufCMessage base;
  ProtobufCBinaryData algorithm;
  int32_t share_size;
  int32_t required_shares;
  int32_t optimal_shares;
  int32_t total_shares;
};
#define STORJ__LIBUPLINK__REDUNDANCY_SCHEME__INIT \
 { PROTOBUF_C_MESSAGE_INIT (&storj__libuplink__redundancy_scheme__descriptor) \
    , {0,NULL}, 0, 0, 0, 0 }


struct  _Storj__Libuplink__BucketConfig
{
  ProtobufCMessage base;
  ProtobufCBinaryData path_cipher;
  Storj__Libuplink__EncryptionParameters *encryption_parameters;
  Storj__Libuplink__RedundancyScheme *redundancy_scheme;
  uint64_t segment_size;
};
#define STORJ__LIBUPLINK__BUCKET_CONFIG__INIT \
 { PROTOBUF_C_MESSAGE_INIT (&storj__libuplink__bucket_config__descriptor) \
    , {0,NULL}, NULL, NULL, 0 }


/* Storj__Libuplink__IDVersion methods */
void   storj__libuplink__idversion__init
                     (Storj__Libuplink__IDVersion         *message);
size_t storj__libuplink__idversion__get_packed_size
                     (const Storj__Libuplink__IDVersion   *message);
size_t storj__libuplink__idversion__pack
                     (const Storj__Libuplink__IDVersion   *message,
                      uint8_t             *out);
size_t storj__libuplink__idversion__pack_to_buffer
                     (const Storj__Libuplink__IDVersion   *message,
                      ProtobufCBuffer     *buffer);
Storj__Libuplink__IDVersion *
       storj__libuplink__idversion__unpack
                     (ProtobufCAllocator  *allocator,
                      size_t               len,
                      const uint8_t       *data);
void   storj__libuplink__idversion__free_unpacked
                     (Storj__Libuplink__IDVersion *message,
                      ProtobufCAllocator *allocator);
/* Storj__Libuplink__TLSConfig methods */
void   storj__libuplink__tlsconfig__init
                     (Storj__Libuplink__TLSConfig         *message);
size_t storj__libuplink__tlsconfig__get_packed_size
                     (const Storj__Libuplink__TLSConfig   *message);
size_t storj__libuplink__tlsconfig__pack
                     (const Storj__Libuplink__TLSConfig   *message,
                      uint8_t             *out);
size_t storj__libuplink__tlsconfig__pack_to_buffer
                     (const Storj__Libuplink__TLSConfig   *message,
                      ProtobufCBuffer     *buffer);
Storj__Libuplink__TLSConfig *
       storj__libuplink__tlsconfig__unpack
                     (ProtobufCAllocator  *allocator,
                      size_t               len,
                      const uint8_t       *data);
void   storj__libuplink__tlsconfig__free_unpacked
                     (Storj__Libuplink__TLSConfig *message,
                      ProtobufCAllocator *allocator);
/* Storj__Libuplink__UplinkConfig methods */
void   storj__libuplink__uplink_config__init
                     (Storj__Libuplink__UplinkConfig         *message);
size_t storj__libuplink__uplink_config__get_packed_size
                     (const Storj__Libuplink__UplinkConfig   *message);
size_t storj__libuplink__uplink_config__pack
                     (const Storj__Libuplink__UplinkConfig   *message,
                      uint8_t             *out);
size_t storj__libuplink__uplink_config__pack_to_buffer
                     (const Storj__Libuplink__UplinkConfig   *message,
                      ProtobufCBuffer     *buffer);
Storj__Libuplink__UplinkConfig *
       storj__libuplink__uplink_config__unpack
                     (ProtobufCAllocator  *allocator,
                      size_t               len,
                      const uint8_t       *data);
void   storj__libuplink__uplink_config__free_unpacked
                     (Storj__Libuplink__UplinkConfig *message,
                      ProtobufCAllocator *allocator);
/* Storj__Libuplink__EncryptionParameters methods */
void   storj__libuplink__encryption_parameters__init
                     (Storj__Libuplink__EncryptionParameters         *message);
size_t storj__libuplink__encryption_parameters__get_packed_size
                     (const Storj__Libuplink__EncryptionParameters   *message);
size_t storj__libuplink__encryption_parameters__pack
                     (const Storj__Libuplink__EncryptionParameters   *message,
                      uint8_t             *out);
size_t storj__libuplink__encryption_parameters__pack_to_buffer
                     (const Storj__Libuplink__EncryptionParameters   *message,
                      ProtobufCBuffer     *buffer);
Storj__Libuplink__EncryptionParameters *
       storj__libuplink__encryption_parameters__unpack
                     (ProtobufCAllocator  *allocator,
                      size_t               len,
                      const uint8_t       *data);
void   storj__libuplink__encryption_parameters__free_unpacked
                     (Storj__Libuplink__EncryptionParameters *message,
                      ProtobufCAllocator *allocator);
/* Storj__Libuplink__RedundancyScheme methods */
void   storj__libuplink__redundancy_scheme__init
                     (Storj__Libuplink__RedundancyScheme         *message);
size_t storj__libuplink__redundancy_scheme__get_packed_size
                     (const Storj__Libuplink__RedundancyScheme   *message);
size_t storj__libuplink__redundancy_scheme__pack
                     (const Storj__Libuplink__RedundancyScheme   *message,
                      uint8_t             *out);
size_t storj__libuplink__redundancy_scheme__pack_to_buffer
                     (const Storj__Libuplink__RedundancyScheme   *message,
                      ProtobufCBuffer     *buffer);
Storj__Libuplink__RedundancyScheme *
       storj__libuplink__redundancy_scheme__unpack
                     (ProtobufCAllocator  *allocator,
                      size_t               len,
                      const uint8_t       *data);
void   storj__libuplink__redundancy_scheme__free_unpacked
                     (Storj__Libuplink__RedundancyScheme *message,
                      ProtobufCAllocator *allocator);
/* Storj__Libuplink__BucketConfig methods */
void   storj__libuplink__bucket_config__init
                     (Storj__Libuplink__BucketConfig         *message);
size_t storj__libuplink__bucket_config__get_packed_size
                     (const Storj__Libuplink__BucketConfig   *message);
size_t storj__libuplink__bucket_config__pack
                     (const Storj__Libuplink__BucketConfig   *message,
                      uint8_t             *out);
size_t storj__libuplink__bucket_config__pack_to_buffer
                     (const Storj__Libuplink__BucketConfig   *message,
                      ProtobufCBuffer     *buffer);
Storj__Libuplink__BucketConfig *
       storj__libuplink__bucket_config__unpack
                     (ProtobufCAllocator  *allocator,
                      size_t               len,
                      const uint8_t       *data);
void   storj__libuplink__bucket_config__free_unpacked
                     (Storj__Libuplink__BucketConfig *message,
                      ProtobufCAllocator *allocator);
/* --- per-message closures --- */

typedef void (*Storj__Libuplink__IDVersion_Closure)
                 (const Storj__Libuplink__IDVersion *message,
                  void *closure_data);
typedef void (*Storj__Libuplink__TLSConfig_Closure)
                 (const Storj__Libuplink__TLSConfig *message,
                  void *closure_data);
typedef void (*Storj__Libuplink__UplinkConfig_Closure)
                 (const Storj__Libuplink__UplinkConfig *message,
                  void *closure_data);
typedef void (*Storj__Libuplink__EncryptionParameters_Closure)
                 (const Storj__Libuplink__EncryptionParameters *message,
                  void *closure_data);
typedef void (*Storj__Libuplink__RedundancyScheme_Closure)
                 (const Storj__Libuplink__RedundancyScheme *message,
                  void *closure_data);
typedef void (*Storj__Libuplink__BucketConfig_Closure)
                 (const Storj__Libuplink__BucketConfig *message,
                  void *closure_data);

/* --- services --- */


/* --- descriptors --- */

extern const ProtobufCMessageDescriptor storj__libuplink__idversion__descriptor;
extern const ProtobufCMessageDescriptor storj__libuplink__tlsconfig__descriptor;
extern const ProtobufCMessageDescriptor storj__libuplink__uplink_config__descriptor;
extern const ProtobufCMessageDescriptor storj__libuplink__encryption_parameters__descriptor;
extern const ProtobufCMessageDescriptor storj__libuplink__redundancy_scheme__descriptor;
extern const ProtobufCMessageDescriptor storj__libuplink__bucket_config__descriptor;

PROTOBUF_C__END_DECLS


#endif  /* PROTOBUF_C_uplink_2eproto__INCLUDED */
