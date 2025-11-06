// 获取 DOM 元素
const dropArea = document.getElementById('drop-area');
const fileInput = document.getElementById('file-input');
const selectFilesBtn = document.getElementById('select-files-btn');
const uploadButton = document.getElementById('upload-button');
const statusDiv = document.getElementById('status');
const fileListContainer = document.getElementById('file-list-container');
const refreshListButton = document.getElementById('refresh-list');
const progressFill = document.getElementById('progress-fill');
const progressText = document.getElementById('progress-text');

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
                    renderFileList(response.files);
                } catch (error) {
                    console.error('解析文件列表出错:', error);
                }
            } else {
                console.error('获取文件列表失败，状态码:', xhr.status);
            }
        }
    };
    
    xhr.addEventListener('error', function() {
        console.error('网络请求出错');
    });
    
    xhr.send();
}

// 渲染文件列表
function renderFileList(files) {
    // 清空现有列表
    fileListContainer.innerHTML = '';
    
    if (files.length === 0) {
        fileListContainer.innerHTML = '<tr><td colspan="5" class="empty-list">暂无文件</td></tr>';
        return;
    }
    
    // 添加每个文件到列表
    files.forEach(file => {
        const row = document.createElement('tr');
        
        const fileNameCell = document.createElement('td');
        fileNameCell.className = 'file-name';
        fileNameCell.textContent = file.name;
        
        const fileSizeCell = document.createElement('td');
        fileSizeCell.className = 'file-size';
        fileSizeCell.textContent = formatFileSize(file.size);
        
        const modTimeCell = document.createElement('td');
        modTimeCell.className = 'mod-time';
        modTimeCell.textContent = new Date(file.mod_time).toLocaleString();
        
        const md5Cell = document.createElement('td');
        md5Cell.className = 'file-md5';
        md5Cell.textContent = file.md5 ? file.md5.substring(0, 8) : 'N/A';
        if (file.md5) {
            md5Cell.title = file.md5;
        }
        
        const actionCell = document.createElement('td');
        const downloadLink = document.createElement('a');
        downloadLink.className = 'download-link';
        downloadLink.innerHTML = '<i class="fas fa-download"></i> 下载';
        downloadLink.href = `/download/${encodeURIComponent(file.name)}`;
        downloadLink.target = '_blank';
        actionCell.appendChild(downloadLink);
        
        row.appendChild(fileNameCell);
        row.appendChild(fileSizeCell);
        row.appendChild(modTimeCell);
        row.appendChild(md5Cell);
        row.appendChild(actionCell);
        
        fileListContainer.appendChild(row);
    });
}

// 格式化文件大小
function formatFileSize(bytes) {
    if (bytes === 0) return '0 Bytes';
    
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

// 页面加载完成后获取文件列表
document.addEventListener('DOMContentLoaded', function() {
    fetchFileList();
});

// 刷新按钮点击事件
refreshListButton.addEventListener('click', fetchFileList);

// 上传按钮点击事件
uploadButton.addEventListener('click', uploadFiles);