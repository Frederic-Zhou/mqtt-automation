package models

// Command 命令结构
type Command struct {
	ID        string `json:"id"`        // 命令唯一ID
	Command   string `json:"command"`   // 要执行的命令
	Timeout   int    `json:"timeout"`   // 超时时间(秒)，默认30秒
	SerialNo  string `json:"serialno"`  // 设备序列号
	Timestamp int64  `json:"timestamp"` // 时间戳
}

// Response 响应结构
type Response struct {
	ID        string `json:"id"`        // 对应的命令ID
	Status    string `json:"status"`    // success, error, timeout
	Output    string `json:"output"`    // 命令输出
	Error     string `json:"error"`     // 错误信息（如果有）
	Duration  int64  `json:"duration"`  // 执行耗时(毫秒)
	Timestamp int64  `json:"timestamp"` // 时间戳
}
