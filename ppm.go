/*	-- References for writing this library
	The original documentation that started it all: https://github.com/pbsds/hatena-server/wiki/PPM-format
	Used for testing to parse the first frame's first layer with a white paper and a black pen: https://gist.github.com/jaames/aa77713839c1dc948eefd445442bf606
	Python parser to help shape the code: https://github.com/jaames/flipnote-tools/blob/master/scripts/dsi/ppmTools/ppmTools/ppmParser.py#L190
	JavaScript parser to help shape the code: https://github.com/jaames/flipnote.js/blob/master/src/decoder/index.js#L304
*/
package ppm

import (
	//"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	//"io"
	//"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/bovarysme/adpcm"
)

var (
	Debug = false
	ppmMagic = []byte("PARA")
	regexID = "[0159]{1}[0-9A-F]{6}0[0-9A-F]{8}"
	regexFileName = "[0-9A-F]{6}_[0-9A-F]{13}_[0-9]{3}"
	thumbnailPalette = []color.RGBA{0x0: color.RGBA{255, 255, 255, 255}, // Not used/white
		0x1: color.RGBA{84, 84, 84, 255}, // Dark Grey
		0x2: color.RGBA{255, 255, 255, 255}, // White
		0x3: color.RGBA{165, 165, 165, 255}, // Light Grey
		0x4: color.RGBA{255, 0, 0, 255}, // Pure Red
		0x5: color.RGBA{128, 0, 0, 255}, // Dark Red
		0x6: color.RGBA{255, 128, 128, 255}, // Light Red/Pink
		0x7: color.RGBA{0, 255, 0, 255}, // Pure Green
		0x8: color.RGBA{0, 0, 255, 255}, // Pure Blue
		0x9: color.RGBA{0, 0, 128, 255}, // Dark Blue
		0xA: color.RGBA{128, 128, 255, 255}, // Light Blue
		0xB: color.RGBA{0, 255, 0, 255}, // Pure Green
		0xC: color.RGBA{255, 0, 255, 255}, // Magenta/Purple
		0xD: color.RGBA{0, 255, 0, 255}, // Pure Green
		0xE: color.RGBA{0, 255, 0, 255}, // Pure Green
		0xF: color.RGBA{0, 255, 0, 255}} // Pure Green
	framePalette = map[string]color.RGBA{"black": color.RGBA{14, 14, 14, 255}, // Black
		"white": color.RGBA{255, 255, 255, 255}, // White
		"blue": color.RGBA{10, 57, 255, 255}, // Blue
		"red": color.RGBA{255, 42, 42, 255}} // Red
)

type PPM struct {
	AuthorName string
	Date int64
	FileName string
	FrameData FrameData
	LastEditedAuthorID string
	LastEditedAuthorName string
	Locked bool
	OriginalAuthorID string
	OriginalAuthorName string
	OriginalFileName string
	PartialFileName string
	PreviousEditingAuthorID string
	SoundData SoundData
	
	// Extra things
	Success bool
	FileLocation string
	OpenConfig *OpenConfig
	ThumbnailPalette []color.RGBA
}
type OpenConfig struct {
	SkipAnimationSize bool
	SkipAuthorName bool
	SkipAudioData bool
	SkipAudioSize bool
	SkipDate bool
	SkipFileName bool
	SkipFileNameCheck bool
	SkipFrameCount bool
	SkipFrameData bool
	SkipFrames bool
	SkipLastEditedAuthorID bool
	SkipLastEditedAuthorIDCheck bool
	SkipLastEditedAuthorName bool
	SkipLockStatus bool
	SkipMagicCheck bool
	SkipOriginalAuthorID bool
	SkipOriginalAuthorIDCheck bool
	SkipOriginalAuthorName bool
	SkipOriginalFileName bool
	SkipOriginalFileNameCheck bool
	SkipPartialFileName bool
	SkipPreviewFrameN bool
	SkipPreviousEditingAuthorID bool
	SkipPreviousEditingAuthorIDCheck bool
	SkipThumbnail bool
}
type FrameData struct {
	FrameCount int
	FrameOffsets []uint32
	Frames []Frame
	PreviewFrameBitmap []byte
	PreviewFrameImage image.Image
	PreviewFrame int
	Size int
}
type Frame struct {
	FrameImage image.Image
}
type unpackedFrame struct {
	Frame [2][192][256]byte
	FrameOffset uint32
	IsNewFrame bool
	IsTranslated bool
	TranslateX int
	TranslateY int
	PaperColor byte
	PenColor []byte
	HasFrame bool
}

type Offset struct {
	Offset uint32
	Length int
}

type SoundData struct {
	SoundMeta SoundMeta
	BGM []int // PCM audio
	SoundEffect1 []int // PCM audio
	SoundEffect2 []int // PCM audio
	SoundEffect3 []int // PCM audio
	Size int
}
type SoundMeta struct {
	BGM Offset
	SoundEffect1 Offset
	SoundEffect2 Offset
	SoundEffect3 Offset
	FrameSpeed int
	BGMSpeed int
}

func debugLog(msg string) {
	if Debug {
		fmt.Println(msg)
	}
}

func (ppmData *PPM) Open() (error) {
	ppmFile, err := os.Open(ppmData.FileLocation)
	if err != nil {
		return err
	}
	defer ppmFile.Close()
	
	magic := make([]byte, 4)
	ppmFile.ReadAt(magic, 0x0)
	if !bytes.Equal(ppmMagic, magic) {
		return errors.New("PPM magic incorrect")
	}

	audioSize := make([]byte, 4)
	ppmFile.ReadAt(audioSize, 0x8)
	ppmData.SoundData.Size = hex2int(binaryReadLE(audioSize))

	lockStatus := make([]byte, 2)
	ppmFile.ReadAt(lockStatus, 0x10)
	if hex2int(lockStatus) > 0 {
		ppmData.Locked = true
	} else {
		ppmData.Locked = false
	}

	originalAuthorName := make([]byte, 22)
	ppmFile.ReadAt(originalAuthorName, 0x14)
	ppmData.OriginalAuthorName = hex2string(originalAuthorName, true)

	lastEditedAuthorName := make([]byte, 22)
	ppmFile.ReadAt(lastEditedAuthorName, 0x2A)
	ppmData.LastEditedAuthorName = hex2string(lastEditedAuthorName, true)

	authorName := make([]byte, 22)
	ppmFile.ReadAt(authorName, 0x40)
	ppmData.AuthorName = hex2string(authorName, true)

	originalAuthorIDBytes := make([]byte, 8)
	ppmFile.ReadAt(originalAuthorIDBytes, 0x56)
	originalAuthorID := binaryReadLE(originalAuthorIDBytes)
	ppmData.OriginalAuthorID = strings.ToUpper(hexAsString(originalAuthorID))
	regexpIDMatch, _ := regexp.MatchString(regexID, ppmData.OriginalAuthorID)
	if !regexpIDMatch { return errors.New("Original author ID is not valid") }

	lastEditedAuthorIDBytes := make([]byte, 8)
	ppmFile.ReadAt(lastEditedAuthorIDBytes, 0x5E)
	lastEditedAuthorID := binaryReadLE(lastEditedAuthorIDBytes)
	ppmData.LastEditedAuthorID = strings.ToUpper(hexAsString(lastEditedAuthorID))
	regexpIDMatch, _ = regexp.MatchString(regexID, ppmData.LastEditedAuthorID)
	if !regexpIDMatch { return errors.New("Last edited author ID is not valid") }
	
	previousEditingAuthorIDBytes := make([]byte, 8)
	ppmFile.ReadAt(previousEditingAuthorIDBytes, 0x8A)
	previousEditingAuthorID := binaryReadLE(previousEditingAuthorIDBytes)
	ppmData.PreviousEditingAuthorID = strings.ToUpper(hexAsString(previousEditingAuthorID))
	regexpIDMatch, _ = regexp.MatchString(regexID, ppmData.PreviousEditingAuthorID)
	if !regexpIDMatch { return errors.New("Previous editing author ID is not valid") }

	originalFileName1 := make([]byte, 3)
	originalFileName2 := make([]byte, 13)
	originalFileName3 := make([]byte, 2)
	ppmFile.ReadAt(originalFileName1, 0x66)
	ppmFile.ReadAt(originalFileName2, 0x69)
	ppmFile.ReadAt(originalFileName3, 0x7C)

	ppmData.OriginalFileName = strings.ToUpper(hexAsString(originalFileName1) + "_" + hex2string(originalFileName2, false) + "_" + padLeft(strconv.Itoa(hex2int(originalFileName3)), "0", 3))
	regexpFileNameMatch, _ := regexp.MatchString(regexFileName, ppmData.OriginalFileName)
	if !regexpFileNameMatch { return errors.New("Original file name is not valid") }

	fileName1 := make([]byte, 3)
	fileName2 := make([]byte, 13)
	fileName3 := make([]byte, 2)
	ppmFile.ReadAt(fileName1, 0x78)
	ppmFile.ReadAt(fileName2, 0x7B)
	ppmFile.ReadAt(fileName3, 0x8E)
		
	ppmData.FileName = strings.ToUpper(hexAsString(fileName1) + "_" + hex2string(fileName2, false) + "_" + padLeft(strconv.Itoa(hex2int(fileName3)), "0", 3))
	regexpFileNameMatch, _ = regexp.MatchString(regexFileName, ppmData.FileName)
	if !regexpFileNameMatch { return errors.New("File name is not valid") }

	partialFileName := make([]byte, 8)
	ppmFile.ReadAt(partialFileName, 0x92)
	ppmData.PartialFileName = hex2string(partialFileName, false)

	date := make([]byte, 4)
	ppmFile.ReadAt(date, 0x9A)
	ppmData.Date = (hex2int64(date) + 946684800)

	animationSize := make([]byte, 4)
	ppmFile.ReadAt(animationSize, 0x4)
	ppmData.FrameData.Size = int(binaryReadLE_uint32(animationSize))

	frameCountBytes := make([]byte, 2)
	ppmFile.ReadAt(frameCountBytes, 0xC)
	frameCount := int(binaryReadLE_uint16(frameCountBytes)) + 1
	if frameCount > 999 {
		ppmData.FrameData.FrameCount = 999
	} else {
		ppmData.FrameData.FrameCount = frameCount
	}

	previewFrameN := make([]byte, 2)
	ppmFile.ReadAt(previewFrameN, 0x12)
	ppmData.FrameData.PreviewFrame = int(binaryReadLE_uint8(previewFrameN))

	previewBitmap := make([]byte, 1536)
	ppmFile.ReadAt(previewBitmap, 0xA0)
	ppmData.FrameData.PreviewFrameBitmap = previewBitmap
			
	previewImage := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for tileY := 0; tileY < 6; tileY++ {
		for tileX := 0; tileX < 8; tileX++ {
			for imageY := 0; imageY < 8; imageY++ {
				for imageX := 0; imageX < 8; imageX += 2 {
					colorLoc := (tileY * 512 + tileX * 64 + imageY * 8 + imageX) / 2
					colorByte := previewBitmap[colorLoc]
					color1 := singleHex2int(colorByte & 0xF)
					color2 := singleHex2int(colorByte >> 4)
					rgbaColor1 := thumbnailPalette[color1]
					rgbaColor2 := thumbnailPalette[color2]
					previewImage.Set(imageX + tileX * 8, imageY + tileY * 8, rgbaColor1)
					previewImage.Set(imageX + tileX * 8 + 1, imageY + tileY * 8, rgbaColor2)
				}
			}
		}
	}
	ppmData.FrameData.PreviewFrameImage = previewImage

	if ppmData.FrameData.FrameCount > 0 {
		ppmData.FrameData.Frames = make([]Frame, ppmData.FrameData.FrameCount)
		
		ppmFile.Seek(0x06A0, 0) // Jump to the start of the animation data section
		offsetTableLengthBytes := make([]byte, 2) // Make a byte array to store the offset table length
		ppmFile.Read(offsetTableLengthBytes) // Read the offset table length into the byte array
		offsetTableLength := binaryReadLE_uint16(offsetTableLengthBytes) // Get the uint16 representation of the byte array
				
		debugLog("Offset table length: " + strconv.Itoa(int(offsetTableLength)))
				
		ppmFile.Seek(0x06A8, 0) // Skip padding and unknown
		
		// Read frame offsets and build them into an array of frame offsets
		frameOffsetsSize := ppmData.FrameData.FrameCount // Get the size of the frame offset array
		debugLog("Frame offsets array size: " + strconv.Itoa(int(frameOffsetsSize)))
		frameOffsets := make([]uint32, frameOffsetsSize) // Create the frame offset array and set its value type to uint32
		for frameOffsetN := 0; frameOffsetN < int(frameOffsetsSize); frameOffsetN++ { // Loop through the frame offset array
			frameOffsetBytes := make([]byte, 4) // Make a byte array to store the frame offset
			ppmFile.Read(frameOffsetBytes) // Read the frame offset (relative to the end of the offset table) into the byte array
			frameOffsets[frameOffsetN] = uint32(0x06A8 + offsetTableLength) + binaryReadLE_uint32(frameOffsetBytes) // Store the frame offset (relative to the beginning of the file) in the frame offset array
			debugLog("Frame " + strconv.Itoa(frameOffsetN) + " offset: " + fmt.Sprintf("%v", frameOffsets[frameOffsetN]))
		}
		ppmData.FrameData.FrameOffsets = frameOffsets
				
		currentFrame := &unpackedFrame{}
		prevFrame := &unpackedFrame{}
		for frameN := 0; frameN < int(frameOffsetsSize); frameN++ {
			debugLog("> Parsing frame " + strconv.Itoa(frameN) + " out of " + strconv.Itoa(ppmData.FrameData.FrameCount) + "...")
			
			prevFrame = currentFrame
			currentFrame = decodeFrame(ppmFile, ppmData, frameN, &unpackedFrame{})
			if !currentFrame.IsNewFrame {
				for line := 0; line < 192; line++ {
					for pixelPosition := 0; pixelPosition < 256; pixelPosition++ {
						for layer := 0; layer < 2; layer++ {
							currentFrame.Frame[layer][line][pixelPosition] = currentFrame.Frame[layer][line][pixelPosition]^prevFrame.Frame[layer][line][pixelPosition]
						}
					}
				}
			}
					
			frameImage := getFrameImage(currentFrame, ppmFile, ppmData, frameN)
			ppmData.FrameData.Frames[frameN].FrameImage = frameImage
		}
	}

	debugLog("> Decoding sound header...")
	decodeSoundHeader(ppmFile, ppmData)
	debugLog("> Decoding BGM...")
	ppmData.SoundData.BGM = decodeAudio(ppmFile, ppmData, ppmData.SoundData.SoundMeta.BGM.Offset, ppmData.SoundData.SoundMeta.BGM.Length)
	debugLog("> Decoding SoundEffect1...")
	ppmData.SoundData.SoundEffect1 = decodeAudio(ppmFile, ppmData, ppmData.SoundData.SoundMeta.SoundEffect1.Offset, ppmData.SoundData.SoundMeta.SoundEffect1.Length)
	debugLog("> Decoding SoundEffect2...")
	ppmData.SoundData.SoundEffect2 = decodeAudio(ppmFile, ppmData, ppmData.SoundData.SoundMeta.SoundEffect2.Offset, ppmData.SoundData.SoundMeta.SoundEffect2.Length)
	debugLog("> Decoding SoundEffect3...")
	ppmData.SoundData.SoundEffect3 = decodeAudio(ppmFile, ppmData, ppmData.SoundData.SoundMeta.SoundEffect3.Offset, ppmData.SoundData.SoundMeta.SoundEffect3.Length)
	
	debugLog("> Finished decoding PPM")
	ppmData.Success = true
	return nil
}

func decodeSoundHeader(ppmFile *os.File, ppmData *PPM) {
	soundHeaderOffset := 0x06A0 + ppmData.FrameData.Size + ppmData.FrameData.FrameCount
	if (soundHeaderOffset % 4) != 0 { soundHeaderOffset += 4 - (soundHeaderOffset % 4) }
	ppmFile.Seek(int64(soundHeaderOffset), 0)
	
	bgmSizeBytes := make([]byte, 4)
	sec1SizeBytes := make([]byte, 4)
	sec2SizeBytes := make([]byte, 4)
	sec3SizeBytes := make([]byte, 4)
	ppmFile.Read(bgmSizeBytes)
	ppmFile.Read(sec1SizeBytes)
	ppmFile.Read(sec2SizeBytes)
	ppmFile.Read(sec3SizeBytes)
	bgmSize := binaryReadLE_uint32(bgmSizeBytes)
	sec1Size := binaryReadLE_uint32(sec1SizeBytes)
	sec2Size := binaryReadLE_uint32(sec2SizeBytes)
	sec3Size := binaryReadLE_uint32(sec3SizeBytes)

	frameSpeedBytes := make([]byte, 2)
	ppmFile.Read(frameSpeedBytes)
	frameSpeed := 8 - binaryReadLE_uint8(frameSpeedBytes)

	bgmSpeedBytes := make([]byte, 2)
	ppmFile.Read(bgmSpeedBytes)
	bgmSpeed := 8 - binaryReadLE_uint8(bgmSpeedBytes)

	ppmData.SoundData.SoundMeta.FrameSpeed = int(frameSpeed)
	ppmData.SoundData.SoundMeta.BGMSpeed = int(bgmSpeed)

	soundHeaderOffset += 32
	ppmData.SoundData.SoundMeta.BGM.Offset = uint32(soundHeaderOffset)
	ppmData.SoundData.SoundMeta.BGM.Length = int(bgmSize)
	soundHeaderOffset += int(bgmSize)
	ppmData.SoundData.SoundMeta.SoundEffect1.Offset = uint32(soundHeaderOffset)
	ppmData.SoundData.SoundMeta.SoundEffect1.Length = int(sec1Size)
	soundHeaderOffset += int(sec1Size)
	ppmData.SoundData.SoundMeta.SoundEffect2.Offset = uint32(soundHeaderOffset)
	ppmData.SoundData.SoundMeta.SoundEffect2.Length = int(sec2Size)
	soundHeaderOffset += int(sec2Size)
	ppmData.SoundData.SoundMeta.SoundEffect3.Offset = uint32(soundHeaderOffset)
	ppmData.SoundData.SoundMeta.SoundEffect3.Length = int(sec3Size)
}

func decodeAudio(ppmFile *os.File, ppmData *PPM, trackOffset uint32, trackLength int) []int {
	debugLog("> Decoding track at offset " + strconv.Itoa(int(trackOffset)) + " with length " + strconv.Itoa(trackLength))

	ppmFile.Seek(int64(trackOffset), 0)

	buffer := make([]byte, trackLength)
	ppmFile.Read(buffer)
	for i := 0; i < trackLength; i++ {
		buffer[i] = (buffer[i] & 0xF) << 4 | (buffer[i] >> 4) // Flipnote Studio's adpcm data uses reverse nibble order
	}
	audio := make([]int, 0)
	decoder := adpcm.NewDecoder(1)
	decoder.Decode(buffer, &audio)
	return audio
}

func decodeSoundFlags(ppmFile *os.File, ppmData *PPM) [][3]byte {
	ppmFile.Seek(int64(0x06A0 + ppmData.FrameData.Size), 0)
	array := make([][3]byte, ppmData.FrameData.FrameCount)
	for i := 0; i < ppmData.FrameData.FrameCount; i++ {
		newByteBytes := make([]byte, 2)
		ppmFile.Read(newByteBytes)
		newByte := binaryReadLE_uint8(newByteBytes)
		array[i][0] = newByte & 0x1
		array[i][1] = (newByte >> 1) & 0x1
		array[i][2] = (newByte >> 2) & 0x1
	}
	return array
}

func decodeFrame(ppmFile *os.File, ppmData *PPM, frameN int, prevFrame *unpackedFrame) *unpackedFrame {
	frameOffset := ppmData.FrameData.FrameOffsets[frameN]
	ppmFile.Seek(int64(frameOffset), 0) // Jump to the current frame
	
	frameHeaderBytes := make([]byte, 1)
	ppmFile.Read(frameHeaderBytes)
	frameHeader := uint(frameHeaderBytes[0])
	isNewFrame := false
	if ((frameHeader >> 7) & 0x1) > 0 { isNewFrame = true }
	isTranslated := false
	if ((frameHeader >> 5) & 0x3) > 0 { isTranslated = true }
	translateX := 0
	translateY := 0
	if isTranslated {
		translateXBytes := make([]byte, 1)
		translateYBytes := make([]byte, 1)
		ppmFile.Read(translateXBytes)
		ppmFile.Read(translateYBytes)
		translateX = int(translateXBytes[0])
		translateY = int(translateYBytes[0])
	}
	paperColor := byte(frameHeader & 0x1)
	penColor := make([]byte, 2)
	penColor[0] = byte((frameHeader >> 1) & 0x3)
	penColor[1] = byte((frameHeader >> 3) & 0x3)
	
	layer1LineEncodingsBytes := make([]byte, 48)
	layer2LineEncodingsBytes := make([]byte, 48)
	layerLineEncodings := [2][192]uint{}
	ppmFile.Read(layer1LineEncodingsBytes)
	ppmFile.Read(layer2LineEncodingsBytes)
	for byteOffset := 0; byteOffset < 48; byteOffset++ {
		layer1LineEncoding := uint(layer1LineEncodingsBytes[byteOffset])
		layer2LineEncoding := uint(layer2LineEncodingsBytes[byteOffset])
		for bitOffset := 0; bitOffset < 8; bitOffset += 2 {
			uBitOffset := uint(bitOffset)
			layerLineEncodings[0][byteOffset * 4 + bitOffset / 2] = (layer1LineEncoding >> uBitOffset) & 0x3
			layerLineEncodings[1][byteOffset * 4 + bitOffset / 2] = (layer2LineEncoding >> uBitOffset) & 0x3
		}
	}
	
	frame := [2][192][256]byte{}
	
	for layer := 0; layer < 2; layer++ {
		for line := 0; line < 192; line++ {
			lineType := layerLineEncodings[layer][line] & 0x3
			
			switch lineType {
				case 0:
					continue
				case 1:
					lineHeaderBytes := make([]byte, 4)
					ppmFile.Read(lineHeaderBytes)
					lineHeader := hex2uint32(lineHeaderBytes)
					
					pixelPosition := 0
					for (lineHeader & 0xFFFFFFFF > 0) {
						if (lineHeader & 0x80000000 > 0) {
							chunkByte := make([]byte, 1)
							ppmFile.Read(chunkByte)
							chunkByteInt := uint(chunkByte[0])
							for loop := 0; loop < 8; loop++ {
								if (chunkByteInt & 0x1) == 1 {
									frame[layer][line][pixelPosition] = penColor[layer]
								}
								pixelPosition += 1
								chunkByteInt = chunkByteInt >> 1
							}
						} else {
							pixelPosition += 8
						}
						lineHeader = lineHeader << 1
					}
				case 2:
					lineHeaderBytes := make([]byte, 4)
					ppmFile.Read(lineHeaderBytes)
					lineHeader := hex2uint32(lineHeaderBytes)
					
					for pixelPosition := 0; pixelPosition < 256; pixelPosition++ {
						frame[layer][line][pixelPosition] = penColor[layer]
					}
					
					pixelPosition := 0
					for (lineHeader & 0xFFFFFFFF > 0) {
						if (lineHeader & 0x80000000 > 0) {
							chunkByte := make([]byte, 1)
							ppmFile.Read(chunkByte)
							chunkByteInt := uint(chunkByte[0])
							for loop := 0; loop < 8; loop++ {
								if (chunkByteInt & 0x1) == 0 {
									frame[layer][line][pixelPosition] = paperColor
								}
								pixelPosition += 1
								chunkByteInt = chunkByteInt >> 1
							}
						} else {
							pixelPosition += 8
						}
						lineHeader = lineHeader << 1
					}
				case 3:
					lineDataBytes := make([]byte, 32)
					ppmFile.Read(lineDataBytes)
					
					pixelPosition := 0
					for lineDataIndex := 0; lineDataIndex < 32; lineDataIndex++ {
						chunkByte := uint(lineDataBytes[lineDataIndex])
						for loop := 0; loop < 8; loop++ {
							if (chunkByte & 0x1) == 1 {
								frame[layer][line][pixelPosition] = byte(chunkByte >> uint(pixelPosition) & 0x1)
							}
							pixelPosition += 1
							chunkByte = chunkByte >> 1
						}
					}
			}
		}
	}
	
	unpackedFrame := &unpackedFrame{Frame:frame,FrameOffset:frameOffset,IsNewFrame:isNewFrame,IsTranslated:isTranslated,TranslateX:translateX,TranslateY:translateY,PaperColor:paperColor,PenColor:penColor,HasFrame:true}
	return unpackedFrame
}

func decodePrevFrames(ppmFile *os.File, ppmData *PPM, frameN int) *unpackedFrame {
	backTrack := 0
	isNewFrame := true
	for !isNewFrame {
		backTrack += 1
		backTrackFrame := decodeFrame(ppmFile, ppmData, frameN - backTrack, &unpackedFrame{})
		isNewFrame = backTrackFrame.IsNewFrame
	}
	backTrack = frameN - backTrack
	backTrackFrame := &unpackedFrame{}
	for backTrack < frameN {
		backTrackFrame = decodeFrame(ppmFile, ppmData, backTrack, backTrackFrame)
		backTrack += 1
	}
	return backTrackFrame
}

func getFrameImage(decodedFrame *unpackedFrame, ppmFile *os.File, ppmData *PPM, frameN int) image.Image {
	frame := decodedFrame.Frame
	isNewFrame := decodedFrame.IsNewFrame
	isTranslated := decodedFrame.IsTranslated
	translateX := decodedFrame.TranslateX
	translateY := decodedFrame.TranslateY
	paperColor := decodedFrame.PaperColor
	penColor := decodedFrame.PenColor
	if !isNewFrame {
		//prevDecodedFrame := decodeFrame(ppmFile, ppmData, frameN - 1)
		//frameImage := getFrameImage(prevDecodedFrame, ppmFile, ppmData, frameN - 1).(*image.RGBA)
		frameImage := image.NewRGBA(image.Rect(0, 0, 256, 192))
		for line := 0; line < 256; line++ {
			for pixelPosition := 0; pixelPosition < 192; pixelPosition++ {
				y := line
				x := pixelPosition
				if isTranslated {
					y := line + translateY
					x := pixelPosition + translateX
					if y > 256 { y -= 256 }
					if x > 192 { x -= 192 }
				}
				for layer := 1; layer >= 0; layer-- {
					switch frame[layer][pixelPosition][line] {
						case 0x0:
							if frame[0][pixelPosition][line] == 0x1 {
								if penColor[0] == 0x1 {
									if paperColor == 0x0 {
										frameImage.Set(y, x, framePalette["white"])
									} else {
										frameImage.Set(y, x, framePalette["black"])
									}
								} else if penColor[0] == 0x2 {
									frameImage.Set(y, x, framePalette["red"])
								} else if penColor[0] == 0x3 {
									frameImage.Set(y, x, framePalette["blue"])
								}
							} else if frame[0][pixelPosition][line] == 0x0 && frame[1][pixelPosition][line] == 0x1 {
								if penColor[1] == 0x1 {
									if paperColor == 0x0 {
										frameImage.Set(y, x, framePalette["white"])
									} else {
										frameImage.Set(y, x, framePalette["black"])
									}
								} else if penColor[1] == 0x2 {
									frameImage.Set(y, x, framePalette["red"])
								} else if penColor[1] == 0x3 {
									frameImage.Set(y, x, framePalette["blue"])
								}
							} else {
								if paperColor == 0x0 {
									frameImage.Set(y, x, framePalette["black"])
								} else {
									frameImage.Set(y, x, framePalette["white"])
								}
							}
						case 0x1:
							if paperColor == 0x0 {
								frameImage.Set(y, x, framePalette["white"])
							} else {
								frameImage.Set(y, x, framePalette["black"])
							}
						case 0x2:
							frameImage.Set(y, x, framePalette["red"])
						case 0x3:
							frameImage.Set(y, x	, framePalette["blue"])
					}
				}
			}
		}
		return frameImage
	} else {
		frameImage := image.NewRGBA(image.Rect(0, 0, 256, 192))
		for line := 0; line < 256; line++ {
			for pixelPosition := 0; pixelPosition < 192; pixelPosition++ {
				if paperColor == 0x0 {
					frameImage.Set(line, pixelPosition, framePalette["black"])
				} else {
					frameImage.Set(line, pixelPosition, framePalette["white"])
				}
				for layer := 1; layer >= 0; layer-- {
					switch frame[layer][pixelPosition][line] {
						case 0x1:
							if paperColor == 0x0 {
								frameImage.Set(line, pixelPosition, framePalette["white"])
							} else {
								frameImage.Set(line, pixelPosition, framePalette["black"])
							}
						case 0x2:
							frameImage.Set(line, pixelPosition, framePalette["red"])
						case 0x3:
							frameImage.Set(line, pixelPosition, framePalette["blue"])
					}
				}
			}
		}
		return frameImage
	}
}

func binaryReadLE(byteArray []byte) []byte {
	for i, j := 0, len(byteArray) - 1; i < j; i, j = i + 1, j - 1 {
		byteArray[i], byteArray[j] = byteArray[j], byteArray[i]
	}

	return byteArray
}

func binaryReadLE_uint8(byteArray []byte) uint8 {
	var result uint8
	buffer := bytes.NewReader(byteArray)
	binary.Read(buffer, binary.LittleEndian, &result)
	return result
}
func binaryReadLE_uint16(byteArray []byte) uint16 {
	var result uint16
	buffer := bytes.NewReader(byteArray)
	binary.Read(buffer, binary.LittleEndian, &result)
	return result
}
func binaryReadLE_uint32(byteArray []byte) uint32 {
	var result uint32
	buffer := bytes.NewReader(byteArray)
	binary.Read(buffer, binary.LittleEndian, &result)
	return result
}
func hex2int(hexBytes []byte) int {
	hexStr := "0x" + hex.EncodeToString(hexBytes)
	result, _ := strconv.ParseInt(hexStr, 0, 64)
	return int(result)
}
func hex2uint8(hexBytes []byte) uint8 {
	hexStr := "0x" + hex.EncodeToString(hexBytes)
	result, _ := strconv.ParseUint(hexStr, 0, 8)
	return uint8(result)
}
func hex2uint16(hexBytes []byte) uint16 {
	hexStr := "0x" + hex.EncodeToString(hexBytes)
	result, _ := strconv.ParseUint(hexStr, 0, 16)
	return uint16(result)
}
func hex2uint32(hexBytes []byte) uint32 {
	hexStr := "0x" + hex.EncodeToString(hexBytes)
	result, _ := strconv.ParseUint(hexStr, 0, 64)
	return uint32(result)
}
func hex2int64(hexBytes []byte) int64 {
	hexStr := "0x" + hex.EncodeToString(hexBytes)
	result, _ := strconv.ParseInt(hexStr, 0, 64)
	return result
}
func hex2string(hexBytes []byte, replaceZeroes bool) string {
	hexStr := hex.EncodeToString(hexBytes)
	if replaceZeroes { hexStr = strings.Replace(hexStr, "00", "", -1) }
	hexBytes = []byte(hexStr)
	stringBytes := make([]byte, hex.DecodedLen(len(hexBytes)))
	stringBytesCount, _ := hex.Decode(stringBytes, hexBytes)
	return fmt.Sprintf("%s", stringBytes[:stringBytesCount])
}
func hexAsString(hexBytes []byte) string {
	hexStr := hex.EncodeToString(hexBytes)
	return hexStr
}
func singleHex2int(hexByte byte) int {
	hexBytes := []byte{hexByte}
	return hex2int(hexBytes)
}
func singleHexAsString(hexByte byte) string {
	hexBytes := []byte{hexByte}
	hexStr := hex.EncodeToString(hexBytes)
	return hexStr
}
func padLeft(str, pad string, length int) string {
	for {
		str = pad + str
		if len(str) > length {
			return str[0:length]
		}
	}
}