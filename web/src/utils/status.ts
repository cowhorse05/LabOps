export function statusColor(status: string) {
  switch (status) {
    case 'online':
    case 'success':
      return 'green';
    case 'running':
      return 'blue';
    case 'pending':
      return 'gold';
    case 'failed':
    case 'timeout':
      return 'red';
    case 'offline':
      return 'default';
    default:
      return 'default';
  }
}

export function statusText(status: string) {
  const map: Record<string, string> = {
    online: '在线',
    offline: '离线',
    pending: '等待中',
    running: '运行中',
    success: '成功',
    failed: '失败',
    timeout: '超时',
  };
  return map[status] || status;
}
