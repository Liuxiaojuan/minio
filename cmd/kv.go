package cmd

/*
#include <stdio.h>
#include <stdlib.h>
#include "nkv_api.h"
#include "nkv_result.h"

struct minio_nkv_handle {
  uint64_t nkv_handle;
  uint64_t container_hash;
  uint64_t network_path_hash;
};

static int minio_nkv_open(char *config, uint64_t *nkv_handle) {
  uint64_t instance_uuid = 0;
  nkv_result result;
  result = nkv_open(config, "minio", "msl-ssg-sk01", 1023, &instance_uuid, nkv_handle);
  return result;
}

static int minio_nkv_open_path(struct minio_nkv_handle *handle, char *ipaddr) {
  uint32_t index = 0;
  uint32_t cnt_count = NKV_MAX_ENTRIES_PER_CALL;
  nkv_container_info *cntlist = malloc(sizeof(nkv_container_info)*NKV_MAX_ENTRIES_PER_CALL);
  memset(cntlist, 0, sizeof(nkv_container_info) * NKV_MAX_ENTRIES_PER_CALL);

  for (int i = 0; i < NKV_MAX_ENTRIES_PER_CALL; i++) {
    cntlist[i].num_container_transport = NKV_MAX_CONT_TRANSPORT;
    cntlist[i].transport_list = malloc(sizeof(nkv_container_transport)*NKV_MAX_CONT_TRANSPORT);
    memset(cntlist[i].transport_list, 0, sizeof(nkv_container_transport)*NKV_MAX_CONT_TRANSPORT);
  }

  int result = nkv_physical_container_list (handle->nkv_handle, index, cntlist, &cnt_count);
  if (result != 0) {
    printf("NKV getting physical container list failed !!, error = %d\n", result);
    exit(1);
  }

  nkv_io_context io_ctx[16];
  memset(io_ctx, 0, sizeof(nkv_io_context) * 16);
  uint32_t io_ctx_cnt = 0;

  for (uint32_t i = 0; i < cnt_count; i++) {
    io_ctx[io_ctx_cnt].container_hash = cntlist[i].container_hash;

    for (int p = 0; p < cntlist[i].num_container_transport; p++) {
      printf("Transport information :: hash = %lu, id = %d, address = %s, port = %d, family = %d, speed = %d, status = %d, numa_node = %d\n",
              cntlist[i].transport_list[p].network_path_hash, cntlist[i].transport_list[p].network_path_id, cntlist[i].transport_list[p].ip_addr,
              cntlist[i].transport_list[p].port, cntlist[i].transport_list[p].addr_family, cntlist[i].transport_list[p].speed,
              cntlist[i].transport_list[p].status, cntlist[i].transport_list[p].numa_node);
      io_ctx[io_ctx_cnt].is_pass_through = 1;
      io_ctx[io_ctx_cnt].container_hash = cntlist[i].container_hash;
      io_ctx[io_ctx_cnt].network_path_hash = cntlist[i].transport_list[p].network_path_hash;
      if(!strcmp(cntlist[i].transport_list[p].ip_addr, ipaddr)) {
              handle->container_hash = cntlist[i].container_hash;
              handle->network_path_hash = cntlist[i].transport_list[p].network_path_hash;
              return 0;
      }
      io_ctx_cnt++;
    }
  }
  return 1;
}

static int minio_nkv_put(struct minio_nkv_handle *handle, void *key, int keyLen, void *value, int valueLen) {
  nkv_result result;
  nkv_io_context ctx;
  ctx.is_pass_through = 1;
  ctx.container_hash = handle->container_hash;
  ctx.network_path_hash = handle->network_path_hash;
  ctx.ks_id = 0;

  const nkv_key  nkvkey = {key, keyLen};
  nkv_store_option option = {0};
  nkv_value nkvvalue = {value, valueLen, 0};
  result = nkv_store_kvp(handle->nkv_handle, &ctx, &nkvkey, &option, &nkvvalue);
  return result;
}

static int minio_nkv_get(struct minio_nkv_handle *handle, void *key, int keyLen, void *value, int valueLen, int *actual_length) {
  nkv_result result;
  nkv_io_context ctx;
  ctx.is_pass_through = 1;
  ctx.container_hash = handle->container_hash;
  ctx.network_path_hash = handle->network_path_hash;
  ctx.ks_id = 0;

  const nkv_key  nkvkey = {key, keyLen};
  nkv_retrieve_option option = {0};

  nkv_value nkvvalue = {value, valueLen, 0};
  result = nkv_retrieve_kvp(handle->nkv_handle, &ctx, &nkvkey, &option, &nkvvalue);
  *actual_length = nkvvalue.actual_length;
  return result;
}

static int minio_nkv_delete(struct minio_nkv_handle *handle, void *key, int keyLen) {
  nkv_result result;
  nkv_io_context ctx;
  ctx.is_pass_through = 1;
  ctx.container_hash = handle->container_hash;
  ctx.network_path_hash = handle->network_path_hash;
  ctx.ks_id = 0;

  const nkv_key  nkvkey = {key, keyLen};
  result = nkv_delete_kvp(handle->nkv_handle, &ctx, &nkvkey);
  return result;
}

typedef struct minio_nkv_private_ {
  void *pfn;
  uint64_t channel;
  int actual_length;
  nkv_io_context ctx;
  nkv_key  nkvkey;
  nkv_value nkvvalue;
  nkv_store_option store_option;
  nkv_retrieve_option retrieve_option;
} minio_nkv_private;

extern void minio_nkv_callback(void *, int);

static void nkv_aio_complete (nkv_aio_construct* op_data, int32_t num_op) {
  minio_nkv_private* pvt = (minio_nkv_private*) op_data->private_data_1;
  if (!op_data) {
    printf("NKV Async IO returned NULL op_Data, ignoring..");
  }
  if (op_data->result == 0 && op_data->opcode == 0) {
    pvt->actual_length = op_data->value.actual_length;
  }
  minio_nkv_callback((void*)pvt->channel, op_data->result);
  free(pvt->pfn);
}

static int minio_nkv_put_async(struct minio_nkv_handle *handle, uint64_t pvtmasked) {
  struct minio_nkv_private_ *pvt = (struct minio_nkv_private_ *) pvtmasked;
  nkv_postprocess_function* pfn = (nkv_postprocess_function*) malloc (sizeof(nkv_postprocess_function));
  pfn->nkv_aio_cb = nkv_aio_complete;
  pfn->private_data_1 = (void*)pvt;
  pvt->pfn = pfn;

  pvt->ctx.is_pass_through = 1;
  pvt->ctx.container_hash = handle->container_hash;
  pvt->ctx.network_path_hash = handle->network_path_hash;
  pvt->ctx.ks_id = 0;

  nkv_result result = nkv_store_kvp_async(handle->nkv_handle, &pvt->ctx, &pvt->nkvkey, &pvt->store_option, &pvt->nkvvalue, pfn);
  return result;
}

static int minio_nkv_get_async(struct minio_nkv_handle *handle, uint64_t pvtmasked) {
  struct minio_nkv_private_ *pvt = (struct minio_nkv_private_ *) pvtmasked;
  nkv_postprocess_function* pfn = (nkv_postprocess_function*) malloc (sizeof(nkv_postprocess_function));

  pfn->nkv_aio_cb = nkv_aio_complete;
  pfn->private_data_1 = (void*)pvt;
  pvt->pfn = pfn;

  pvt->ctx.is_pass_through = 1;
  pvt->ctx.container_hash = handle->container_hash;
  pvt->ctx.network_path_hash = handle->network_path_hash;
  pvt->ctx.ks_id = 0;

  nkv_result result = nkv_retrieve_kvp_async(handle->nkv_handle, &pvt->ctx, &pvt->nkvkey, &pvt->retrieve_option, &pvt->nkvvalue, pfn);
  return result;
}

static int minio_nkv_delete_async(struct minio_nkv_handle *handle, uint64_t pvtmasked) {
  struct minio_nkv_private_ *pvt = (struct minio_nkv_private_ *) pvtmasked;
  nkv_postprocess_function* pfn = (nkv_postprocess_function*) malloc (sizeof(nkv_postprocess_function));

  pfn->nkv_aio_cb = nkv_aio_complete;
  pfn->private_data_1 = (void*)pvt;
  pvt->pfn = pfn;

  pvt->ctx.is_pass_through = 1;
  pvt->ctx.container_hash = handle->container_hash;
  pvt->ctx.network_path_hash = handle->network_path_hash;
  pvt->ctx.ks_id = 0;

  nkv_result result = nkv_delete_kvp_async(handle->nkv_handle, &pvt->ctx, &pvt->nkvkey, pfn);
  return result;
}

*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
	"unsafe"
)

//export minio_nkv_callback
func minio_nkv_callback(chanPtr unsafe.Pointer, result C.int) {
	c := *(*chan int)(chanPtr)
	select {
	case c <- int(result):
	case <-time.After(kvTimeout):
		fmt.Println("No listeners for result chan")
		os.Exit(1)
	}
}

var kvTimeout time.Duration = func() time.Duration {
	timeoutStr := os.Getenv("MINIO_NKV_TIMEOUT")
	if timeoutStr == "" {
		return time.Duration(10) * time.Second
	}
	i, err := strconv.Atoi(timeoutStr)
	if err != nil {
		fmt.Println("MINIO_NKV_TIMEOUT is incorrect", timeoutStr, err)
		os.Exit(1)
	}
	return time.Duration(i) * time.Second
}()

var globalNKVHandle C.uint64_t

func minio_nkv_open(configPath string) error {
	if globalNKVHandle != 0 {
		return nil
	}
	go kvAsyncLoop()
	cs := C.CString(configPath)
	status := C.minio_nkv_open(cs, &globalNKVHandle)
	C.free(unsafe.Pointer(cs))
	if status != 0 {
		return errDiskNotFound
	}
	return nil
}

func newKV(path string, sync bool) (*KV, error) {
	kv := &KV{}
	kv.path = path
	kv.sync = sync
	kv.handle.nkv_handle = globalNKVHandle
	cs := C.CString(path)
	status := C.minio_nkv_open_path(&kv.handle, C.CString(path))
	C.free(unsafe.Pointer(cs))
	if status != 0 {
		fmt.Println("unable to open", path, status)
		return nil, errors.New("unable to open device")
	}
	return kv, nil
}

type KV struct {
	handle C.struct_minio_nkv_handle
	path   string
	sync   bool
}

var kvValuePool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, kvMaxValueSize)
		return &b
	},
}

const kvKeyLength = 200

var kvMu sync.Mutex
var kvSerialize = os.Getenv("MINIO_NKV_SERIALIZE") != ""

type kvCallType int

const (
	kvCallPut kvCallType = iota
	kvCallGet
	kvCallDel
)

type asyncKVRequest struct {
	call   kvCallType
	path   string
	key    string
	ch     chan int
	handle *C.struct_minio_nkv_handle
	pvt    *C.struct_minio_nkv_private_
}

var globalAsyncKVRequestCh chan asyncKVRequest
var globalKVNilChan chan C.struct_minio_nkv_private_

func kvAsyncLoop() {
	globalAsyncKVRequestCh = make(chan asyncKVRequest)
	var requests []asyncKVRequest
	for request := range globalAsyncKVRequestCh {
		switch request.call {
		case kvCallPut:
			requests = append(requests, request)
			fmt.Println("Put", request.path, request.key)
			C.minio_nkv_put_async(request.handle, C.ulong(uintptr(unsafe.Pointer(request.pvt))))
			fmt.Println("Put", request.path, request.key, "done")
		case kvCallGet:
			requests = append(requests, request)
			fmt.Println("Get", request.path, request.key)
			C.minio_nkv_get_async(request.handle, C.ulong(uintptr(unsafe.Pointer(request.pvt))))
			fmt.Println("Get", request.path, request.key, "done")
		case kvCallDel:
			requests = append(requests, request)
			fmt.Println("Delete", request.path, request.key)
			C.minio_nkv_delete_async(request.handle, C.ulong(uintptr(unsafe.Pointer(request.pvt))))
			fmt.Println("Delete", request.path, request.key, "done")
		}
	}
}

func (k *KV) Put(keyStr string, value []byte) error {
	if kvSerialize {
		kvMu.Lock()
		defer kvMu.Unlock()
	}
	if len(value) > kvMaxValueSize {
		return errValueTooLong
	}
	key := []byte(keyStr)
	for len(key) < kvKeyLength {
		key = append(key, '\x00')
	}
	if len(key) > kvKeyLength {
		fmt.Println("invalid key length", key, len(key))
		os.Exit(0)
	}
	var valuePtr unsafe.Pointer
	if len(value) > 0 {
		valuePtr = unsafe.Pointer(&value[0])
	}
	var status int
	if k.sync {
		cstatus := C.minio_nkv_put(&k.handle, unsafe.Pointer(&key[0]), C.int(len(key)), valuePtr, C.int(len(value)))
		status = int(cstatus)
	} else {
		pvt := C.struct_minio_nkv_private_{}
		c := make(chan int, 1)
		pvt.channel = C.ulong(uintptr(unsafe.Pointer(&c)))
		pvt.nkvkey.key = unsafe.Pointer(&key[0])
		pvt.nkvkey.length = C.uint(len(key))
		pvt.nkvvalue.value = valuePtr
		pvt.nkvvalue.length = C.ulong(len(value))
		select {
		case globalAsyncKVRequestCh <- asyncKVRequest{kvCallPut, k.path, keyStr, c, &k.handle, &pvt}:
		case <-time.After(kvTimeout):
			fmt.Println("Put timeout on globalAsyncKVRequestCh", k.path, keyStr)
			os.Exit(1)
		}
		status = 1
		select {
		case <-time.After(kvTimeout):
			fmt.Println("Put timeout", k.path, keyStr)
			os.Exit(1)
			return errDiskNotFound
		case status = <-c:
		case globalKVNilChan <- pvt:
		}
	}

	if status != 0 {
		return errors.New("error during put")
	}
	return nil
}

func (k *KV) Get(keyStr string, value []byte) ([]byte, error) {
	if kvSerialize {
		kvMu.Lock()
		defer kvMu.Unlock()
	}
	key := []byte(keyStr)
	for len(key) < kvKeyLength {
		key = append(key, '\x00')
	}
	if len(key) > kvKeyLength {
		fmt.Println("invalid key length", key, len(key))
		os.Exit(0)
	}
	var actualLength C.int

	tries := 10
	for {
		status := 1
		if k.sync {
			cstatus := C.minio_nkv_get(&k.handle, unsafe.Pointer(&key[0]), C.int(len(key)), unsafe.Pointer(&value[0]), C.int(len(value)), &actualLength)
			status = int(cstatus)
		} else {
			c := make(chan int, 1)
			pvt := C.struct_minio_nkv_private_{}
			pvt.channel = C.ulong(uintptr(unsafe.Pointer(&c)))
			pvt.nkvkey.key = unsafe.Pointer(&key[0])
			pvt.nkvkey.length = C.uint(len(key))
			pvt.nkvvalue.value = unsafe.Pointer(&value[0])
			pvt.nkvvalue.length = C.ulong(len(value))
			select {
			case globalAsyncKVRequestCh <- asyncKVRequest{kvCallGet, k.path, keyStr, c, &k.handle, &pvt}:
			case <-time.After(kvTimeout):
				fmt.Println("Get timeout on globalAsyncKVRequestCh", k.path, keyStr)
				os.Exit(1)
			}

			status = 1
			select {
			case <-time.After(kvTimeout + 2):
				fmt.Println("Get timeout", k.path, keyStr)
				os.Exit(1)
				return nil, errDiskNotFound
			case status = <-c:
			case globalKVNilChan <- pvt:
			}

			if status == 0 {
				actualLength = pvt.actual_length
			}
		}
		if status != 0 {
			return nil, errFileNotFound
		}
		if actualLength > 0 {
			break
		}
		tries--
		if tries == 0 {
			fmt.Println("GET failed (after 10 retries) on (actual_length=0)", k.path, keyStr)
			os.Exit(1)
		}
	}
	return value[:actualLength], nil
}

func (k *KV) Delete(keyStr string) error {
	if kvSerialize {
		kvMu.Lock()
		defer kvMu.Unlock()
	}
	key := []byte(keyStr)
	for len(key) < kvKeyLength {
		key = append(key, '\x00')
	}
	if len(key) > kvKeyLength {
		fmt.Println("invalid key length", key, len(key))
		os.Exit(0)
	}
	var status int
	if k.sync {
		cstatus := C.minio_nkv_delete(&k.handle, unsafe.Pointer(&key[0]), C.int(len(key)))
		status = int(cstatus)
	} else {
		pvt := C.struct_minio_nkv_private_{}
		c := make(chan int, 1)
		pvt.channel = C.ulong(uintptr(unsafe.Pointer(&c)))
		pvt.nkvkey.key = unsafe.Pointer(&key[0])
		pvt.nkvkey.length = C.uint(len(key))
		select {
		case globalAsyncKVRequestCh <- asyncKVRequest{kvCallDel, k.path, keyStr, c, &k.handle, &pvt}:
		case <-time.After(kvTimeout):
			fmt.Println("Delete timeout on globalAsyncKVRequestCh", k.path, keyStr)
			os.Exit(1)
		}

		status = 1
		select {
		case <-time.After(kvTimeout):
			fmt.Println("Delete timeout", k.path, keyStr)
			os.Exit(1)
			return errDiskNotFound
		case status = <-c:
		case globalKVNilChan <- pvt:
		}

	}
	if status != 0 {
		return errFileNotFound
	}
	return nil
}
