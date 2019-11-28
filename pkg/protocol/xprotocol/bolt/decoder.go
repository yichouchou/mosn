package bolt

import (
	"context"
	"sofastack.io/sofa-mosn/pkg/types"
	"encoding/binary"
	"sofastack.io/sofa-mosn/pkg/buffer"
	"fmt"
)

func decodeRequest(ctx context.Context, data types.IoBuffer, oneway bool) (cmd interface{}, err error) {
	bytesLen := data.Len()
	bytes := data.Bytes()

	// 1. least bytes to decode header is RequestHeaderLen(22)
	if bytesLen < RequestHeaderLen {
		return
	}

	// 2. least bytes to decode whole frame
	classLen := binary.BigEndian.Uint16(bytes[14:16])
	headerLen := binary.BigEndian.Uint16(bytes[16:18])
	contentLen := binary.BigEndian.Uint32(bytes[18:22])

	frameLen := RequestHeaderLen + int(classLen) + int(headerLen) + int(contentLen)
	if bytesLen < frameLen {
		return
	}
	data.Drain(frameLen)

	// 3. decode header
	request := &Request{
		RequestHeader: RequestHeader{
			Protocol:   ProtocolCode,
			CmdType:    CmdTypeRequest,
			CmdCode:    binary.BigEndian.Uint16(bytes[2:4]),
			Version:    bytes[4],
			RequestId:  binary.BigEndian.Uint32(bytes[5:9]),
			Codec:      bytes[9],
			Timeout:    int32(binary.BigEndian.Uint32(bytes[10:14])),
			ClassLen:   classLen,
			HeaderLen:  headerLen,
			ContentLen: contentLen,
		},
		rawData: buffer.GetBytesByContext(ctx, frameLen),
	}
	if oneway {
		request.CmdType = CmdTypeRequestOneway
	}

	//4. copy data for io multiplexing
	copy(*request.rawData, bytes)
	request.Data = buffer.NewIoBufferBytes(*request.rawData)

	//5. process wrappers: Class, Header, Content, Data
	headerIndex := RequestHeaderLen + int(classLen)
	contentIndex := headerIndex + int(headerLen)

	request.rawMeta = (*request.rawData)[:RequestHeaderLen]
	if classLen > 0 {
		request.rawClass = (*request.rawData)[RequestHeaderLen:headerIndex]
		request.Class = string(request.rawClass)
	}
	if headerLen > 0 {
		request.rawHeader = (*request.rawData)[headerIndex:contentIndex]
		err = decodeHeader(request.rawHeader, request.header)
	}
	if contentLen > 0 {
		request.rawContent = (*request.rawData)[contentIndex:]
		request.Content = buffer.NewIoBufferBytes(request.rawContent)
	}
	return request, err
}

func decodeResponse(ctx context.Context, data types.IoBuffer) (cmd interface{}, err error) {
	bytesLen := data.Len()
	bytes := data.Bytes()

	// 1. least bytes to decode header is ResponseHeaderLen(20)
	if bytesLen < ResponseHeaderLen {
		return
	}

	// 2. least bytes to decode whole frame
	classLen := binary.BigEndian.Uint16(bytes[12:14])
	headerLen := binary.BigEndian.Uint16(bytes[14:16])
	contentLen := binary.BigEndian.Uint32(bytes[16:20])

	frameLen := ResponseHeaderLen + int(classLen) + int(headerLen) + int(contentLen)
	if bytesLen < frameLen {
		return
	}
	data.Drain(frameLen)

	// 3. decode header
	response := &Response{
		ResponseHeader: ResponseHeader{
			Protocol:       ProtocolCode,
			CmdType:        CmdTypeResponse,
			CmdCode:        binary.BigEndian.Uint16(bytes[2:4]),
			Version:        bytes[4],
			RequestId:      binary.BigEndian.Uint32(bytes[5:9]),
			Codec:          bytes[9],
			ResponseStatus: binary.BigEndian.Uint16(bytes[10:12]),
			ClassLen:       classLen,
			HeaderLen:      headerLen,
			ContentLen:     contentLen,
		},
		rawData: buffer.GetBytesByContext(ctx, frameLen),
	}

	//4. copy data for io multiplexing
	copy(*response.rawData, bytes)

	response.Data = buffer.NewIoBufferBytes(*response.rawData)

	//5. process wrappers: Class, Header, Content, Data
	headerIndex := RequestHeaderLen + int(classLen)
	contentIndex := headerIndex + int(headerLen)

	response.rawMeta = (*response.rawData)[:RequestHeaderLen]
	if classLen > 0 {
		response.rawClass = (*response.rawData)[RequestHeaderLen:headerIndex]
		response.Class = string(response.rawClass)
	}
	if headerLen > 0 {
		response.rawHeader = (*response.rawData)[headerIndex:contentIndex]
		err = decodeHeader(response.rawHeader, response.header)
	}
	if contentLen > 0 {
		response.rawContent = (*response.rawData)[contentIndex:]
		response.Content = buffer.NewIoBufferBytes(response.rawContent)
	}
	return response, err
}

func decodeHeader(bytes []byte, h header) (err error) {
	totalLen := len(bytes)
	index := 0

	for index < totalLen {
		kv := bytesKV{}

		// 1. read key
		kv.key, index, err = decodeStr(bytes, totalLen, index)
		if err != nil {
			return
		}

		// 2. read value
		kv.value, index, err = decodeStr(bytes, totalLen, index)
		if err != nil {
			return
		}

		// 3. kv append
		h.kvs = append(h.kvs, kv)
	}
	return nil
}

func decodeStr(bytes []byte, totalLen, index int) (str []byte, newIndex int, err error) {
	// 1. read str length
	length := binary.BigEndian.Uint32(bytes[index:])
	end := index + 4 + int(length)

	if end > totalLen {
		return nil, end, fmt.Errorf("decode bolt header failed, index %d, length %d, totalLen %d, bytes %v\n", index, length, totalLen, bytes)
	}

	// 2. read str value
	return bytes[index+4 : end], end, nil
}
