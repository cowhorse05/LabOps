import { Component, type ErrorInfo, type ReactNode } from 'react';
import { Button, Result } from 'antd';
import { useLogStore } from '@/stores/logStore';

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
  error: Error | null;
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null };

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('ErrorBoundary caught:', error, info);
    try {
      useLogStore.getState().push({
        level: 'error',
        source: 'ErrorBoundary',
        message: error.message || '未知渲染错误',
        detail: `组件栈:\n${info.componentStack ?? '无'}`,
      });
    } catch {
      // store unavailable during catastrophic failure
    }
  }

  render() {
    if (this.state.hasError) {
      return (
        <Result
          status="error"
          title="页面渲染出错"
          subTitle={this.state.error?.message || '未知错误'}
          extra={
            <Button
              type="primary"
              onClick={() => {
                this.setState({ hasError: false, error: null });
                window.location.reload();
              }}
            >
              重新加载
            </Button>
          }
        />
      );
    }
    return this.props.children;
  }
}
