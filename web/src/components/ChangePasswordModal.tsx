import { useEffect, useState } from 'react';
import { App, Form, Input, Modal } from 'antd';
import { LockOutlined } from '@ant-design/icons';
import { authApi } from '@/api/labops';
import { useAuthStore } from '@/stores/auth';
import type { AxiosError } from 'axios';

interface ChangePasswordModalProps {
  open: boolean;
  onClose: () => void;
}

export default function ChangePasswordModal({ open, onClose }: ChangePasswordModalProps) {
  const [form] = Form.useForm();
  const { message } = App.useApp();
  const updateToken = useAuthStore((s) => s.updateToken);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (open) form.resetFields();
  }, [open, form]);

  async function handleOk() {
    try {
      const values = await form.validateFields();
      setLoading(true);
      const result = await authApi.changePassword(values.oldPassword, values.newPassword);
      updateToken(result.token);
      message.success('密码修改成功');
      onClose();
    } catch (err) {
      if ((err as AxiosError<{ error?: string }>)?.response?.data?.error) {
        message.error((err as AxiosError<{ error?: string }>).response!.data.error!);
      } else if ((err as { errorFields?: unknown[] })?.errorFields) {
        // form validation error — don't show message, Ant Design shows inline
        return;
      }
    } finally {
      setLoading(false);
    }
  }

  return (
    <Modal
      title="修改密码"
      open={open}
      onOk={handleOk}
      onCancel={onClose}
      confirmLoading={loading}
      okText="确认修改"
      cancelText="取消"
      destroyOnClose
    >
      <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
        <Form.Item
          name="oldPassword"
          rules={[{ required: true, message: '请输入当前密码' }]}
        >
          <Input.Password prefix={<LockOutlined />} placeholder="当前密码" />
        </Form.Item>
        <Form.Item
          name="newPassword"
          rules={[
            { required: true, message: '请输入新密码' },
            { min: 4, message: '密码至少 4 个字符' },
          ]}
        >
          <Input.Password prefix={<LockOutlined />} placeholder="新密码（至少 4 个字符）" />
        </Form.Item>
        <Form.Item
          name="confirmPassword"
          dependencies={['newPassword']}
          rules={[
            { required: true, message: '请确认新密码' },
            ({ getFieldValue }) => ({
              validator(_, value) {
                if (!value || getFieldValue('newPassword') === value) return Promise.resolve();
                return Promise.reject(new Error('两次输入的密码不一致'));
              },
            }),
          ]}
        >
          <Input.Password prefix={<LockOutlined />} placeholder="确认新密码" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
