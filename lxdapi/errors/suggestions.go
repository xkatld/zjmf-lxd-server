package errors

var errorSuggestions = map[int]string{
	ERR_SUCCESS: "",

	ERR_CONTAINER_NOT_FOUND:      "请检查容器名称是否正确，或使用 /api/list 查看所有容器",
	ERR_CONTAINER_ALREADY_EXISTS: "容器名称已被使用，请使用其他名称",
	ERR_CONTAINER_RUNNING:        "请先停止容器再执行此操作",
	ERR_CONTAINER_STOPPED:        "请先启动容器",
	ERR_CONTAINER_NOT_RUNNING:    "容器未运行，请检查容器状态",
	ERR_CONTAINER_STATUS_UNKNOWN: "请稍后重试，或联系管理员检查 LXD 服务状态",
	ERR_CONTAINER_CONFIG_INVALID: "请检查容器配置参数是否正确",
	ERR_CONTAINER_NOT_PAUSED:     "容器未处于暂停状态",

	ERR_LXC_CREATE_FAIL:    "请检查镜像是否存在、存储空间是否充足",
	ERR_LXC_START_FAIL:     "请检查容器配置是否正确，或查看系统日志",
	ERR_LXC_STOP_FAIL:      "请尝试强制停止，或联系管理员",
	ERR_LXC_DELETE_FAIL:    "请确保容器已停止，或强制删除",
	ERR_LXC_RESTART_FAIL:   "请检查容器状态，或联系管理员",
	ERR_LXC_EXEC_FAIL:      "请检查命令是否正确，容器内是否有所需工具",
	ERR_LXC_QUERY_FAIL:     "请检查 LXD 服务是否正常运行",
	ERR_LXC_CONFIG_FAIL:    "请检查配置项名称和值是否正确",
	ERR_LXC_SNAPSHOT_FAIL:  "请检查存储空间是否充足",
	ERR_LXC_RESTORE_FAIL:   "请检查快照是否存在",
	ERR_LXC_LIST_FAIL:      "请检查 LXD 服务是否正常，或联系管理员",
	ERR_LXC_PUBLISH_FAIL:   "请检查容器状态和存储空间",
	ERR_LXC_LAUNCH_FAIL:    "请检查镜像名称和网络配置",
	ERR_LXC_COPY_FAIL:      "请检查源容器是否存在，存储空间是否充足",
	ERR_LXC_FILE_PUSH_FAIL: "请检查文件路径和权限",
	ERR_LXC_PAUSE_FAIL:     "请确保容器正在运行",
	ERR_LXC_RESUME_FAIL:    "请确保容器已暂停",

	ERR_NAT_RULE_NOT_FOUND:    "请检查端口号是否正确",
	ERR_NAT_RULE_ADD_FAIL:     "请检查端口是否已被占用，或联系管理员检查防火墙规则",
	ERR_NAT_RULE_DELETE_FAIL:  "请重试，或手动删除 iptables 规则",
	ERR_NAT_IPTABLES_FAIL:     "请检查 iptables 服务是否正常，或查看系统日志",
	ERR_NAT_PORT_IN_USE:       "该端口已被其他容器使用，请选择其他端口",
	ERR_NAT_QUERY_FAIL:        "请检查数据库连接",
	ERR_NAT_CLEANUP_FAIL:      "请手动清理 iptables 规则",
	ERR_NAT_EXTERNAL_IP_EMPTY: "请在配置文件中设置外网 IP 地址",
	ERR_SSH_QUERY_FAIL:        "请检查数据库连接",
	ERR_SSH_CLEANUP_FAIL:      "请手动清理 SSH 端口规则",
	ERR_SSH_RULE_ADD_FAIL:     "请检查端口范围配置",

	ERR_IPV6_BINDING_NOT_FOUND: "请检查 IPv6 地址是否已分配给该容器",
	ERR_IPV6_BINDING_EXISTS:    "该 IPv6 地址已被使用",
	ERR_IPV6_ROUTE_ADD_FAIL:    "请检查网络配置和路由表",
	ERR_IPV6_ROUTE_DELETE_FAIL: "请手动删除路由规则",
	ERR_IPV6_POOL_EXHAUSTED:    "IPv6 地址池已用完，请联系管理员扩容",
	ERR_IPV6_QUERY_FAIL:        "请检查数据库连接",
	ERR_IPV6_CLEANUP_FAIL:      "请手动清理 IPv6 路由规则",
	ERR_IPV6_DISABLED:          "IPv6 功能未启用，请在配置文件中开启",

	ERR_DB_QUERY_FAIL:       "请检查数据库连接和查询语句",
	ERR_DB_INSERT_FAIL:      "请检查数据是否重复或字段是否完整",
	ERR_DB_UPDATE_FAIL:      "请检查记录是否存在",
	ERR_DB_DELETE_FAIL:      "请检查记录是否存在或是否被其他数据引用",
	ERR_DB_TRANSACTION_FAIL: "请重试或联系管理员",
	ERR_DB_RECORD_NOT_FOUND: "请检查查询条件是否正确",
	ERR_DB_DUPLICATE_ENTRY:  "该记录已存在",

	ERR_STORAGE_POOL_NOT_FOUND: "请检查存储池名称，或使用 lxc storage list 查看可用存储池",
	ERR_STORAGE_POOL_FULL:      "请清理不用的容器或镜像，或联系管理员扩容存储",
	ERR_STORAGE_CREATE_FAIL:    "请检查存储池配置和磁盘空间",
	ERR_STORAGE_DELETE_FAIL:    "请确保没有容器在使用该存储",

	ERR_IMAGE_NOT_FOUND:  "请检查镜像名称或使用 lxc image list 查看可用镜像",
	ERR_IMAGE_PULL_FAIL:  "请检查网络连接和镜像源配置",
	ERR_IMAGE_INVALID:    "请使用正确的镜像格式",
	ERR_IMAGE_ALIAS_FAIL: "请检查镜像别名是否已存在",

	ERR_PASSWORD_RESET_FAIL:  "请检查容器是否支持密码重置，或手动登录修改",
	ERR_PASSWORD_SCRIPT_FAIL: "请检查容器内是否有所需工具（如 chpasswd）",
	ERR_PASSWORD_INVALID:     "请使用符合要求的密码（长度、复杂度等）",

	ERR_TASK_CREATE_FAIL:  "请检查任务参数是否正确",
	ERR_TASK_NOT_FOUND:    "请检查任务 ID 是否正确",
	ERR_TASK_ALREADY_RUN:  "任务已在执行中，请等待完成",
	ERR_TASK_TIMEOUT:      "任务执行超时，请检查系统负载或容器配置",
	ERR_TASK_CANCELLED:    "任务已被取消",
	ERR_TASK_RETRY_EXCEED: "任务重试次数已达上限，请检查错误原因",

	ERR_SYSTEM_COMMAND_FAIL: "请检查系统命令和权限",
	ERR_SYSTEM_NETWORK_FAIL: "请检查网络连接",
	ERR_SYSTEM_DISK_FULL:    "磁盘空间不足，请清理磁盘或扩容",
	ERR_SYSTEM_PERMISSION:   "权限不足，请使用管理员权限执行",
	ERR_SYSTEM_TIMEOUT:      "操作超时，请检查系统负载",
	ERR_SYSTEM_UNKNOWN:      "未知错误，请查看系统日志或联系管理员",

	ERR_PROXY_RULE_NOT_FOUND:     "请检查域名是否正确",
	ERR_PROXY_RULE_EXISTS:        "该域名已被使用，请使用其他域名",
	ERR_PROXY_DOMAIN_INVALID:     "请使用有效的域名格式（如 example.com）",
	ERR_PROXY_CONTAINER_NO_IP:    "容器没有 IP 地址，请检查网络配置",
	ERR_PROXY_CONFIG_GENERATE:    "请检查模板文件和配置参数",
	ERR_PROXY_FILE_WRITE:         "请检查文件权限和磁盘空间",
	ERR_PROXY_NGINX_TEST_FAIL:    "Nginx 配置测试失败，请检查配置语法",
	ERR_PROXY_NGINX_RELOAD_FAIL:  "Nginx 重载失败，请手动执行 nginx -s reload",
	ERR_PROXY_DISABLED:           "反向代理功能未启用，请在配置文件中开启",
	ERR_PROXY_TEMPLATE_NOT_FOUND: "模板文件不存在，请检查文件路径",
	ERR_PROXY_TEMPLATE_PARSE:     "模板解析失败，请检查模板语法",
}

func GetSuggestion(errorCode int) string {
	if suggestion, ok := errorSuggestions[errorCode]; ok {
		return suggestion
	}
	return "请查看系统日志获取更多信息，或联系管理员"
}

