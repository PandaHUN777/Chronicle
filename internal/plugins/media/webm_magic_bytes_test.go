// webm_magic_bytes_test.go pins the audio/webm check. The EBML magic bytes
// (0x1A45DFA3) are shared by every Matroska-family file (raw .mkv, WebM
// video and WebM audio), so a declared Content-Type can't be trusted on
// its own; isAudioOnlyWebM does the narrow container parse that tells them
// apart.
//
// Every sample here is genuine, hand-built EBML (not a canned literal):
// buildEBMLHeader/buildTrack assemble real id+size+content element
// sequences the same way a muxer would, using the encode/decode
// counterparts in service.go.

package media

import "testing"

// encodeEBMLID renders id as a big-endian sequence of idLen bytes — the
// same fixed-width form readEBMLID expects (an EBML element id keeps its
// length-marker bits as part of the id itself, unlike a size vint).
func encodeEBMLID(id uint32, idLen int) []byte {
	buf := make([]byte, idLen)
	v := id
	for i := idLen - 1; i >= 0; i-- {
		buf[i] = byte(v)
		v >>= 8
	}
	return buf
}

// encodeEBMLSize renders value as an EBML size vint of exactly length
// bytes — the mirror of readEBMLSize's decode. Callers keep length fixed
// (2) across this file's tiny fixtures, well within a 2-byte vint's
// ~16K range.
func encodeEBMLSize(value uint64, length int) []byte {
	buf := make([]byte, length)
	v := value
	for i := length - 1; i >= 1; i-- {
		buf[i] = byte(v)
		v >>= 8
	}
	marker := byte(0x80) >> uint(length-1)
	buf[0] = marker | byte(v)
	return buf
}

// ebmlElem wraps content in one EBML element header (id, then a 2-byte
// size vint), the building block every fixture below composes.
func ebmlElem(id uint32, idLen int, content []byte) []byte {
	out := encodeEBMLID(id, idLen)
	out = append(out, encodeEBMLSize(uint64(len(content)), 2)...)
	return append(out, content...)
}

// buildTrackEntry returns one TrackEntry element containing a single
// TrackType child of the given Matroska track type (1 = video, 2 = audio).
func buildTrackEntry(trackType byte) []byte {
	typeElem := ebmlElem(ebmlTrackTypeID, 1, []byte{trackType})
	return ebmlElem(ebmlTrackEntryID, 1, typeElem)
}

// buildWebM assembles a minimal but genuine top-level EBML+Segment
// structure: an EBML header (with the given DocType, or none when
// docType == "") followed by a Segment containing a Tracks element with
// one TrackEntry per trackType given.
func buildWebM(docType string, trackTypes ...byte) []byte {
	var out []byte
	if docType != "" {
		docTypeElem := ebmlElem(ebmlDocTypeID, 2, []byte(docType))
		out = append(out, ebmlElem(ebmlHeaderID, 4, docTypeElem)...)
	}
	var tracksContent []byte
	for _, tt := range trackTypes {
		tracksContent = append(tracksContent, buildTrackEntry(tt)...)
	}
	tracksElem := ebmlElem(ebmlTracksID, 4, tracksContent)
	out = append(out, ebmlElem(ebmlSegmentID, 4, tracksElem)...)
	return out
}

func TestIsAudioOnlyWebM_AudioOnly_Accepted(t *testing.T) {
	data := buildWebM("webm", 2)
	if !isAudioOnlyWebM(data) {
		t.Error("a genuine audio-only WebM (DocType webm, TrackType audio) must be accepted")
	}
}

func TestIsAudioOnlyWebM_NoDocType_AudioTrack_Accepted(t *testing.T) {
	// Some encoders omit the EBML header's optional DocType entirely;
	// the track-type check alone must still accept a real audio track.
	data := buildWebM("", 2)
	if !isAudioOnlyWebM(data) {
		t.Error("an audio-only track with no DocType element must still be accepted")
	}
}

func TestIsAudioOnlyWebM_VideoTrack_Rejected(t *testing.T) {
	data := buildWebM("webm", 1)
	if isAudioOnlyWebM(data) {
		t.Error("a WebM video track (TrackType 1) must be rejected under audio/webm")
	}
}

func TestIsAudioOnlyWebM_MixedTracks_Rejected(t *testing.T) {
	// One audio + one video track — the video track alone must sink it.
	data := buildWebM("webm", 2, 1)
	if isAudioOnlyWebM(data) {
		t.Error("a file with any video track must be rejected, even alongside an audio track")
	}
}

func TestIsAudioOnlyWebM_MatroskaDocType_Rejected(t *testing.T) {
	// A raw .mkv shares the exact same magic bytes and can carry the
	// same audio-only track shape; DocType is what actually says "this
	// isn't WebM".
	data := buildWebM("matroska", 2)
	if isAudioOnlyWebM(data) {
		t.Error("DocType matroska (raw .mkv) must be rejected even with an audio-only track")
	}
}

func TestIsAudioOnlyWebM_NoTracksFound_Rejected(t *testing.T) {
	// A Segment with no Tracks element at all — can't confirm audio-only,
	// so this must fail closed rather than fall back to the old
	// magic-bytes-only behavior.
	segment := ebmlElem(ebmlSegmentID, 4, []byte("no tracks here"))
	if isAudioOnlyWebM(segment) {
		t.Error("a file with no discoverable Tracks element must be rejected, not defaulted to allow")
	}
}

func TestIsAudioOnlyWebM_TracksWithNoTrackEntry_Rejected(t *testing.T) {
	// Tracks is present but declares no TrackEntry at all — audioOnly
	// never gets flipped by anything, so this must not silently pass on
	// its untouched true default.
	data := buildWebM("webm")
	if isAudioOnlyWebM(data) {
		t.Error("a Tracks element with no TrackEntry children must be rejected, not defaulted to allow")
	}
}

func TestIsAudioOnlyWebM_Empty_Rejected(t *testing.T) {
	if isAudioOnlyWebM(nil) {
		t.Error("empty input must be rejected")
	}
}

func TestIsAudioOnlyWebM_Truncated_Rejected(t *testing.T) {
	data := buildWebM("webm", 2)
	// Cut it off mid-structure.
	if isAudioOnlyWebM(data[:len(data)-3]) {
		t.Error("a truncated container must be rejected, not accepted on partial data")
	}
}

// bigSizeElem wraps content in an EBML element header using an explicit
// sizeLen, unlike ebmlElem's fixed 2-byte size field (max representable
// length ~16KB, per readEBMLSize's maxVal = 1<<(length*7)-1). The
// budget-exhaustion fixture below needs a Tracks/Segment payload of tens
// of thousands of bytes, well past that, so it sizes those two wrappers
// itself instead of reusing ebmlElem for them.
func bigSizeElem(id uint32, idLen int, content []byte, sizeLen int) []byte {
	out := encodeEBMLID(id, idLen)
	out = append(out, encodeEBMLSize(uint64(len(content)), sizeLen)...)
	return append(out, content...)
}

// TestIsAudioOnlyWebM_BudgetExhaustedBeforeVideoTrack_Rejected: when the
// ebmlMaxScanElements budget runs out before every TrackEntry has been
// inspected, an unvisited entry could be video, so the scan must reject
// the file rather than return its permissive audio-only default.
//
// 5000 padding entries is the smallest count this repo's own fixture
// helpers were confirmed (by bisection against the pre-fix scanner) to
// flip from "correctly rejected" to "bypassed" — the resulting ~35KB file
// is well inside the 10MB upload limit, so this is a practical bypass,
// not a theoretical one.
func TestIsAudioOnlyWebM_BudgetExhaustedBeforeVideoTrack_Rejected(t *testing.T) {
	const paddingEntries = 5000
	trackTypes := make([]byte, 0, paddingEntries+1)
	for i := 0; i < paddingEntries; i++ {
		trackTypes = append(trackTypes, 2) // dummy audio padding
	}
	trackTypes = append(trackTypes, 1) // real video track, placed after the padding

	var tracksContent []byte
	for _, tt := range trackTypes {
		tracksContent = append(tracksContent, buildTrackEntry(tt)...)
	}
	tracksElem := bigSizeElem(ebmlTracksID, 4, tracksContent, 4)
	segElem := bigSizeElem(ebmlSegmentID, 4, tracksElem, 4)
	docTypeElem := ebmlElem(ebmlDocTypeID, 2, []byte("webm"))
	data := append(ebmlElem(ebmlHeaderID, 4, docTypeElem), segElem...)

	if isAudioOnlyWebM(data) {
		t.Error("a video track pushed past the scan budget by audio padding must still sink the file, not be silently skipped")
	}
}

func TestValidateMagicBytes_AudioWebm_RequiresRealAudio(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		want bool
	}{
		{"genuine audio-only webm", buildWebM("webm", 2), true},
		{"webm video rejected", buildWebM("webm", 1), false},
		{"raw mkv rejected", buildWebM("matroska", 2), false},
		{"ebml magic with garbage body", []byte{0x1A, 0x45, 0xDF, 0xA3, 0x00, 0x00}, false},
		{"too short to even have the magic", []byte{0x1A, 0x45}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateMagicBytes(tc.data, "audio/webm"); got != tc.want {
				t.Errorf("validateMagicBytes(%q, audio/webm) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}
