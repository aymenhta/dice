package static

var (
	TxtBytes  = []byte("txt")
	NullBytes = []byte("")
	OKBytes   = []byte("OK")

	RespPrefixedErrorBytes        = []byte("-ERR ")
	RespPrefixedWrongTypeErrBytes = []byte("-WRONGTYPE ")
	RespPrefixedOKBytes           = []byte("+OK")
	RespPrefixedNullStringBytes   = []byte("$-1")
	RespPrefixedNullArrayBytes    = []byte("*-1")
	RespPrefixedBoolTrueBytes     = []byte("#t")
	RespPrefixedBoolFalseBytes    = []byte("#f")
)
