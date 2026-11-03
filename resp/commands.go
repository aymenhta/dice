package resp

func (h *ClientHandler) cmdPing(args [][]byte) error {
	if len(args) > 1 {
		return h.WriteError(wrongArgsNum("ping"))
	}

	if len(args) == 1 {
		return h.WriteBytes(args[0])
	}

	return h.WriteSimpleString("PONG")
}

func (h *ClientHandler) cmdEcho(args [][]byte) error {
	if len(args) != 1 {
		return h.WriteError(wrongArgsNum("echo"))
	}
	return h.WriteBytes(args[0])
}

func (h *ClientHandler) cmdQuit() error {
	return h.WriteSimpleString("OK")
}

func (h *ClientHandler) cmdCommand() error {
	return h.WriteSimpleString("OK")
}
