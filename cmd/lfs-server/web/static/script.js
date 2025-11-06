// 获取 DOM 元素
const dropArea = document.getElementById('drop-area');
const fileInput = document.getElementById('file-input');
const selectFilesBtn = document.getElementById('select-files-btn');
const uploadButton = document.getElementById('upload-button');
const statusDiv = document.getElementById('status');
const fileListContainer = document.getElementById('file-list-container');
const fileTreeContainer = document.getElementById('file-tree-container');
const refreshListButton = document.getElementById('refresh-list');
const progressFill = document.getElementById('progress-fill');
const progressText = document.getElementById('progress-text');

// 聊天室相关元素
const chatMessages = document.getElementById('chat-messages');
const chatInput = document.getElementById('chat-input');
const chatSendBtn = document.getElementById('chat-send-btn');
const chatStatus = document.getElementById('chat-status');

// WebSocket连接
let ws = null;

// 存储选中的文件
let selectedFiles = [];

// 阻止默认的拖放事件
['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
    dropArea.addEventListener(eventName, preventDefaults, false);
    document.body.addEventListener(eventName, preventDefaults, false);
});

function preventDefaults(e) {
    e.preventDefault();
    e.stopPropagation();
}

// 当文件拖入时添加高亮样式
['dragenter', 'dragover'].forEach(eventName => {
    dropArea.addEventListener(eventName, highlightDropArea, false);
});

['dragleave', 'drop'].forEach(eventName => {
    dropArea.addEventListener(eventName, unhighlightDropArea, false);
});

function highlightDropArea() {
    dropArea.classList.add('highlight');
}

function unhighlightDropArea() {
    dropArea.classList.remove('highlight');
}

// 处理文件放下事件
dropArea.addEventListener('drop', handleDrop, false);

function handleDrop(e) {
    unhighlightDropArea();
    const dt = e.dataTransfer;
    const files = dt.files;
    handleFiles(files);
}

// 点击选择文件按钮触发文件选择框
selectFilesBtn.addEventListener('click', triggerFileInput);

function triggerFileInput() {
    fileInput.click();
}

// 处理文件选择框的文件选择事件
fileInput.addEventListener('change', handleFileInputChange);

function handleFileInputChange() {
    const files = fileInput.files;
    handleFiles(files);
}

// 处理选中的文件
function handleFiles(files) {
    selectedFiles = Array.from(files); // 保存选中的文件
    // 显示文件列表
    displaySelectedFiles(selectedFiles);
    
    // 启用上传按钮
    if (selectedFiles.length > 0) {
        uploadButton.disabled = false;
    } else {
        uploadButton.disabled = true;
    }
}

// 显示选中的文件列表
function displaySelectedFiles(files) {
    statusDiv.innerHTML = '';
    if (files.length > 0) {
        const fileListTitle = document.createElement('h3');
        fileListTitle.textContent = `已选择 ${files.length} 个文件:`;
        statusDiv.appendChild(fileListTitle);
        
        const fileList = document.createElement('ul');
        fileList.className = 'selected-file-list';
        
        files.forEach(file => {
            const listItem = document.createElement('li');
            listItem.textContent = `${file.name} (${formatFileSize(file.size)})`;
            fileList.appendChild(listItem);
        });
        
        statusDiv.appendChild(fileList);
    }
}

// 上传文件到后端
function uploadFiles() {
    if (selectedFiles.length === 0) {
        showStatusMessage('请选择要上传的文件', 'error');
        return;
    }

    // 对于大文件，使用分片上传
    const largeFiles = selectedFiles.filter(file => file.size > 10 * 1024 * 1024); // 大于10MB的文件
    const smallFiles = selectedFiles.filter(file => file.size <= 10 * 1024 * 1024); // 小于等于10MB的文件

    let promises = [];
    
    // 上传小文件
    if (smallFiles.length > 0) {
        promises.push(uploadSmallFiles(smallFiles));
    }
    
    // 上传大文件（分片上传）
    largeFiles.forEach(file => {
        promises.push(uploadLargeFile(file));
    });

    Promise.all(promises)
        .then(results => {
            showStatusMessage('所有文件上传完成', 'success');
            fetchFileList(); // 上传完成后刷新文件列表
            // 清空已选择的文件
            selectedFiles = [];
            uploadButton.disabled = true;
        })
        .catch(error => {
            showStatusMessage(`上传失败: ${error.message}`, 'error');
        });
}

// 上传小文件
function uploadSmallFiles(files) {
    return new Promise((resolve, reject) => {
        const formData = new FormData();
        files.forEach(file => {
            formData.append('files', file);
        });

        const xhr = new XMLHttpRequest();
        xhr.open('POST', '/batch-upload', true);

        // 监听上传进度
        xhr.upload.addEventListener('progress', handleUploadProgress);

        // 监听请求状态变化
        xhr.onreadystatechange = function () {
            if (xhr.readyState === 4) {
                if (xhr.status === 200) {
                    try {
                        const response = JSON.parse(xhr.responseText);
                        resolve(response);
                    } catch (error) {
                        reject(new Error('解析响应数据出错'));
                    }
                } else {
                    try {
                        const response = JSON.parse(xhr.responseText);
                        reject(new Error(response.error));
                    } catch (error) {
                        reject(new Error(`上传失败，状态码: ${xhr.status}`));
                    }
                }
            }
        };

        // 监听请求错误
        xhr.addEventListener('error', function() {
            reject(new Error('网络请求出错'));
        });

        xhr.send(formData);
    });
}

// 上传大文件（分片上传）
function uploadLargeFile(file) {
    return new Promise((resolve, reject) => {
        const chunkSize = 2 * 1024 * 1024; // 2MB per chunk
        const chunks = Math.ceil(file.size / chunkSize);
        let currentChunk = 0;

        // 计算文件MD5
        calculateFileMD5(file)
            .then(md5 => {
                uploadNextChunk();
                
                function uploadNextChunk() {
                    const start = currentChunk * chunkSize;
                    const end = Math.min(start + chunkSize, file.size);
                    const chunk = file.slice(start, end);

                    const formData = new FormData();
                    formData.append('fileName', file.name);
                    formData.append('totalSize', file.size);
                    formData.append('chunkIndex', currentChunk);
                    formData.append('chunkSize', chunk.size);
                    formData.append('totalChunk', chunks);
                    formData.append('md5', md5);
                    formData.append('file', chunk, `${file.name}.part${currentChunk}`);

                    const xhr = new XMLHttpRequest();
                    
                    // 监听上传进度
                    xhr.upload.addEventListener('progress', (e) => {
                        if (e.lengthComputable) {
                            const chunkPercent = (e.loaded / e.total) * 100;
                            const totalPercent = (currentChunk + chunkPercent / 100) / chunks * 100;
                            updateProgress(totalPercent);
                        }
                    });

                    xhr.onreadystatechange = function () {
                        if (xhr.readyState === 4) {
                            if (xhr.status === 200) {
                                currentChunk++;
                                if (currentChunk < chunks) {
                                    uploadNextChunk();
                                } else {
                                    resolve({message: `文件 ${file.name} 上传完成`});
                                }
                            } else {
                                try {
                                    const response = JSON.parse(xhr.responseText);
                                    reject(new Error(response.error));
                                } catch (error) {
                                    reject(new Error(`上传失败，状态码: ${xhr.status}`));
                                }
                            }
                        }
                    };

                    xhr.addEventListener('error', function() {
                        reject(new Error('网络请求出错'));
                    });

                    xhr.open('POST', '/upload-chunk', true);
                    xhr.send(formData);
                }
            })
            .catch(error => {
                reject(new Error(`计算文件MD5失败: ${error.message}`));
            });
    });
}

// 计算文件MD5
function calculateFileMD5(file) {
    return new Promise((resolve, reject) => {
        const spark = new SparkMD5.ArrayBuffer();
        const reader = new FileReader();
        const chunkSize = 2 * 1024 * 1024; // 2MB
        let currentChunk = 0;
        const chunks = Math.ceil(file.size / chunkSize);

        reader.onload = function(e) {
            spark.append(e.target.result);
            currentChunk++;

            if (currentChunk < chunks) {
                loadNext();
            } else {
                const md5 = spark.end();
                resolve(md5);
            }
        };

        reader.onerror = function() {
            reject(new Error('读取文件失败'));
        };

        function loadNext() {
            const start = currentChunk * chunkSize;
            const end = Math.min(start + chunkSize, file.size);
            const blob = file.slice(start, end);
            reader.readAsArrayBuffer(blob);
        }

        loadNext();
    });
}

// 更新进度条
function updateProgress(percent) {
    progressFill.style.width = percent + '%';
    progressText.textContent = Math.round(percent) + '%';
}

// 处理上传进度
function handleUploadProgress(e) {
    if (e.lengthComputable) {
        const percentComplete = (e.loaded / e.total) * 100;
        updateProgress(percentComplete);
    }
}

// 显示上传结果
function showUploadResult(response) {
    let message = `
        <div class="upload-result">
            <h3>批量上传完成:</h3>
            <p>总计: ${response.total}</p>
            <p class="success">成功: ${response.success_count}</p>
            <p class="error">失败: ${response.error_count}</p>
        </div>
    `;
    
    if (response.errors && response.errors.length > 0) {
        message += '<h4>错误详情:</h4><ul class="error-list">';
        response.errors.forEach(error => {
            message += `<li>${error}</li>`;
        });
        message += '</ul>';
    }
    
    statusDiv.innerHTML = message;
}

// 显示状态消息
function showStatusMessage(message, type = 'info') {
    statusDiv.innerHTML = `<div class="${type}-message">${message}</div>`;
    
    // 3秒后自动清除消息
    setTimeout(() => {
        statusDiv.innerHTML = '';
        // 重置进度条
        updateProgress(0);
    }, 3000);
}

// 获取文件列表
function fetchFileList() {
    const xhr = new XMLHttpRequest();
    xhr.open('GET', '/files', true);
    
    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4) {
            if (xhr.status === 200) {
                try {
                    const response = JSON.parse(xhr.responseText);
                    renderFileTree(response.files);
                } catch (error) {
                    // 解析失败，静默处理
                }
            }
        }
    };
    
    xhr.addEventListener('error', function() {
        // 网络错误，静默处理
    });
    
    xhr.send();
}

// 渲染文件树
function renderFileTree(files) {
    fileTreeContainer.innerHTML = '';
    
    if (files.length === 0) {
        fileTreeContainer.innerHTML = '<div class="empty-list">暂无文件</div>';
        return;
    }
    
    const tree = document.createElement('ul');
    tree.className = 'file-tree';
    
    files.forEach(file => {
        const item = createFileTreeItem(file);
        tree.appendChild(item);
    });
    
    fileTreeContainer.appendChild(tree);
}

// 创建文件树项
function createFileTreeItem(file) {
    const li = document.createElement('li');
    li.className = `file-tree-item ${file.is_dir ? 'folder' : 'file'}`;
    
    const icon = document.createElement('i');
    icon.className = `file-tree-item-icon fas ${file.is_dir ? 'fa-folder' : 'fa-file'}`;
    
    const nameSpan = document.createElement('span');
    nameSpan.textContent = file.name;
    
    const infoSpan = document.createElement('span');
    infoSpan.style.marginLeft = '10px';
    infoSpan.style.fontSize = '0.85em';
    infoSpan.style.color = '#6c757d';
    
    if (!file.is_dir) {
        infoSpan.textContent = `(${formatFileSize(file.size)})`;
        if (file.md5) {
            infoSpan.textContent += ` - MD5: ${file.md5.substring(0, 8)}`;
            infoSpan.title = file.md5;
        }
    }
    
    li.appendChild(icon);
    li.appendChild(nameSpan);
    li.appendChild(infoSpan);
    
    // 如果是文件夹且有子项，递归创建子项
    if (file.is_dir && file.children && file.children.length > 0) {
        const childrenUl = document.createElement('ul');
        childrenUl.className = 'file-tree-children';
        
        file.children.forEach(child => {
            const childItem = createFileTreeItem(child);
            childrenUl.appendChild(childItem);
        });
        
        // 添加点击事件折叠/展开
        let expanded = false;
        li.addEventListener('click', function(e) {
            e.stopPropagation();
            expanded = !expanded;
            if (expanded) {
                childrenUl.style.display = 'block';
                icon.className = 'file-tree-item-icon fas fa-folder-open';
            } else {
                childrenUl.style.display = 'none';
                icon.className = 'file-tree-item-icon fas fa-folder';
            }
        });
        
        childrenUl.style.display = 'none';
        li.appendChild(childrenUl);
    } else if (!file.is_dir) {
        // 文件添加下载链接
        const downloadLink = document.createElement('a');
        downloadLink.href = `/download/${encodeURIComponent(file.path)}`;
        downloadLink.target = '_blank';
        downloadLink.style.marginLeft = '10px';
        downloadLink.style.color = '#667eea';
        downloadLink.innerHTML = '<i class="fas fa-download"></i>';
        downloadLink.title = '下载';
        li.appendChild(downloadLink);
    }
    
    return li;
}

// 格式化文件大小
function formatFileSize(bytes) {
    if (bytes === 0) return '0 Bytes';
    
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// 初始化WebSocket连接
function initWebSocket() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws/chat`;
    
    ws = new WebSocket(wsUrl);
    
    ws.onopen = function() {
        chatStatus.textContent = '已连接';
        chatStatus.className = 'chat-status connected';
        chatInput.disabled = false;
        chatSendBtn.disabled = false;
    };
    
    ws.onclose = function() {
        chatStatus.textContent = '未连接';
        chatStatus.className = 'chat-status disconnected';
        chatInput.disabled = true;
        chatSendBtn.disabled = true;
        
        // 5秒后尝试重连
        setTimeout(initWebSocket, 5000);
    };
    
    ws.onerror = function(error) {
        chatStatus.textContent = '连接错误';
        chatStatus.className = 'chat-status disconnected';
    };
    
    ws.onmessage = function(event) {
        // json.NewEncoder 会在JSON后添加换行符
        // 处理可能的多行消息（批量发送时）
        const lines = event.data.trim().split('\n').filter(line => line.trim());
        
        for (let line of lines) {
            try {
                const message = JSON.parse(line);
                if (message && typeof message === 'object') {
                    // 确保消息有必要的字段
                    if (!message.type) message.type = 'message';
                    // 确保消息内容存在
                    if (message.message === undefined) message.message = '';
                    addChatMessage(message);
                }
            } catch (error) {
                // 解析失败，忽略单条消息
                continue;
            }
        }
    };
}

// 添加聊天消息到界面
function addChatMessage(message) {
    if (!message) {
        return;
    }
    
    // 每次都重新获取元素，确保DOM已加载
    const messagesContainer = document.getElementById('chat-messages');
    if (!messagesContainer) {
        return;
    }
    
    const messageDiv = document.createElement('div');
    const msgType = message.type || 'message';
    messageDiv.className = `chat-message ${msgType}`;
    
    if (msgType === 'message') {
        // 普通消息：显示昵称、IP、时间和内容
        const header = document.createElement('div');
        header.className = 'chat-message-header';
        
        const nicknameSpan = document.createElement('span');
        nicknameSpan.className = 'chat-message-nickname';
        nicknameSpan.textContent = message.nickname || '未知用户';
        header.appendChild(nicknameSpan);
        
        if (message.ip) {
            header.appendChild(document.createTextNode(' '));
            const ipSpan = document.createElement('span');
            ipSpan.className = 'chat-message-ip';
            ipSpan.textContent = `[${message.ip}]`;
            header.appendChild(ipSpan);
        }
        
        if (message.timestamp) {
            header.appendChild(document.createTextNode(' '));
            const timeSpan = document.createElement('span');
            timeSpan.className = 'chat-message-time';
            timeSpan.textContent = message.timestamp;
            header.appendChild(timeSpan);
        }
        
        const content = document.createElement('div');
        content.className = 'chat-message-content';
        content.textContent = message.message || '';
        
        messageDiv.appendChild(header);
        messageDiv.appendChild(content);
    } else {
        // join/leave 消息：显示消息内容和时间
        const content = document.createElement('div');
        content.textContent = message.message || '';
        messageDiv.appendChild(content);
        
        if (message.timestamp) {
            const timeSpan = document.createElement('div');
            timeSpan.className = 'chat-message-time';
            timeSpan.style.textAlign = 'center';
            timeSpan.style.marginTop = '5px';
            timeSpan.style.fontSize = '0.85em';
            timeSpan.textContent = message.timestamp;
            messageDiv.appendChild(timeSpan);
        }
    }
    
    messagesContainer.appendChild(messageDiv);
    // 滚动到底部
    messagesContainer.scrollTop = messagesContainer.scrollHeight;
}

// 发送聊天消息
function sendChatMessage() {
    const message = chatInput.value.trim();
    if (message && ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({
            type: 'message',
            message: message
        }));
        chatInput.value = '';
    }
}

// 聊天输入框回车发送
chatInput.addEventListener('keypress', function(e) {
    if (e.key === 'Enter') {
        sendChatMessage();
    }
});

// 发送按钮点击事件
chatSendBtn.addEventListener('click', sendChatMessage);

// 页面加载完成后获取文件列表和初始化WebSocket
document.addEventListener('DOMContentLoaded', function() {
    fetchFileList();
    initWebSocket();
});

// 刷新按钮点击事件
refreshListButton.addEventListener('click', fetchFileList);

// 上传按钮点击事件
uploadButton.addEventListener('click', uploadFiles);