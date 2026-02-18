import React, { useState } from 'react';
import { Brain, Eye, EyeOff, CheckCircle, XCircle, Loader2, Trash2 } from 'lucide-react';
import { cn } from '@/lib/utils';

interface GLMConfigProps {
  apiKey: string;
  hasKey: boolean;
  onSave: (key: string) => Promise<boolean>;
  onDelete?: () => Promise<boolean>;
  onTest: (key: string) => Promise<boolean>;
}

export const GLMConfigCard: React.FC<GLMConfigProps> = ({
  apiKey,
  hasKey,
  onSave,
  onDelete,
  onTest,
}) => {
  const [key, setKey] = useState(apiKey);
  const [isVisible, setIsVisible] = useState(false);
  const [isTesting, setIsTesting] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [status, setStatus] = useState<'idle' | 'success' | 'error'>('idle');
  const [statusMessage, setStatusMessage] = useState('');

  const handleTest = async () => {
    if (!key.trim()) {
      setStatus('error');
      setStatusMessage('Please enter an API key first');
      return;
    }

    setIsTesting(true);
    setStatus('idle');

    try {
      const result = await onTest(key);
      if (result) {
        setStatus('success');
        setStatusMessage('Connection successful!');
      } else {
        setStatus('error');
        setStatusMessage('Invalid API key');
      }
    } catch {
      setStatus('error');
      setStatusMessage('Connection failed. Check your API key.');
    } finally {
      setIsTesting(false);
    }
  };

  const handleSave = async () => {
    if (!key.trim()) {
      setStatus('error');
      setStatusMessage('Please enter an API key first');
      return;
    }

    setIsSaving(true);
    setStatus('idle');

    try {
      const result = await onSave(key);
      if (result) {
        setStatus('success');
        setStatusMessage('API key saved successfully');
        setKey(''); // Clear the input after successful save
      } else {
        setStatus('error');
        setStatusMessage('Failed to save API key');
      }
    } catch {
      setStatus('error');
      setStatusMessage('Failed to save API key');
    } finally {
      setIsSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!onDelete) return;

    setIsDeleting(true);
    setStatus('idle');

    try {
      const result = await onDelete();
      if (result) {
        setStatus('success');
        setStatusMessage('API key deleted successfully');
      } else {
        setStatus('error');
        setStatusMessage('Failed to delete API key');
      }
    } catch {
      setStatus('error');
      setStatusMessage('Failed to delete API key');
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <div className="bg-white/5 rounded-xl p-5 border border-white/10">
      <div className="flex items-center justify-between mb-5">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-lg bg-blue-600/20 flex items-center justify-center">
            <Brain className="w-5 h-5 text-blue-500" />
          </div>
          <div>
            <h3 className="text-white font-medium">GLM (Zhipu AI)</h3>
            <p className="text-white/50 text-sm">Chinese LLM Provider</p>
          </div>
        </div>
        <div className={cn(
          'px-3 py-1 rounded-lg text-xs font-medium',
          hasKey ? 'bg-green-500/20 text-green-400 border border-green-500/30' : 'bg-white/5 text-white/50 border border-white/10'
        )}>
          {hasKey ? 'Configured' : 'Not Configured'}
        </div>
      </div>

      <div className="space-y-4">
        <div>
          <label className="text-white/70 text-sm block mb-2">API Key</label>
          <div className="flex gap-2">
            <div className="relative flex-1">
              <input
                type={isVisible ? 'text' : 'password'}
                value={key}
                onChange={(e) => {
                  setKey(e.target.value);
                  setStatus('idle');
                }}
                placeholder={hasKey ? "Enter new key to update..." : "Your GLM API key..."}
                className={cn(
                  "w-full bg-white/5 border rounded-lg px-3 py-2.5 text-white text-sm",
                  "focus:outline-none focus:ring-2 focus:ring-blue-500/50 focus:border-blue-500",
                  "placeholder:text-white/40"
                )}
              />
              <button
                onClick={() => setIsVisible(!isVisible)}
                className="absolute right-2 top-1/2 -translate-y-1/2 p-1.5 hover:bg-white/10 rounded-lg transition-colors"
                type="button"
              >
                {isVisible ? (
                  <EyeOff className="w-4 h-4 text-white/50" />
                ) : (
                  <Eye className="w-4 h-4 text-white/50" />
                )}
              </button>
            </div>
            {hasKey && onDelete && (
              <button
                onClick={handleDelete}
                disabled={isDeleting}
                className={cn(
                  "px-3 py-2.5 rounded-lg text-sm font-medium transition-all flex items-center gap-2",
                  "bg-red-500/20 hover:bg-red-500/30 text-red-400",
                  "disabled:opacity-50 disabled:cursor-not-allowed"
                )}
                type="button"
                title="Delete API key"
              >
                {isDeleting ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : (
                  <Trash2 className="w-4 h-4" />
                )}
              </button>
            )}
          </div>
          <p className="text-white/40 text-xs mt-1.5">
            Get your API key from <a href="https://open.bigmodel.cn" target="_blank" rel="noopener noreferrer" className="text-blue-400 hover:text-blue-300">open.bigmodel.cn</a>
          </p>
        </div>

        <div className="flex gap-2">
          <button
            onClick={handleTest}
            disabled={isTesting || !key.trim()}
            className={cn(
              "px-4 py-2.5 rounded-lg text-sm font-medium transition-all flex items-center gap-2",
              "bg-white/10 hover:bg-white/20 text-white",
              "disabled:opacity-50 disabled:cursor-not-allowed"
            )}
            type="button"
          >
            {isTesting ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                Testing...
              </>
            ) : (
              'Test Connection'
            )}
          </button>
          <button
            onClick={handleSave}
            disabled={isSaving || !key.trim()}
            className={cn(
              "px-4 py-2.5 rounded-lg text-sm font-medium transition-all flex items-center gap-2",
              "bg-blue-600 hover:bg-blue-700 text-white",
              "disabled:opacity-50 disabled:cursor-not-allowed"
            )}
            type="button"
          >
            {isSaving ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                Saving...
              </>
            ) : (
              'Save'
            )}
          </button>
        </div>

        {status === 'success' && (
          <div className="flex items-center gap-2 text-green-400 text-sm bg-green-500/10 border border-green-500/20 rounded-lg px-3 py-2">
            <CheckCircle className="w-4 h-4" />
            {statusMessage}
          </div>
        )}
        {status === 'error' && (
          <div className="flex items-center gap-2 text-red-400 text-sm bg-red-500/10 border border-red-500/20 rounded-lg px-3 py-2">
            <XCircle className="w-4 h-4" />
            {statusMessage}
          </div>
        )}
      </div>

      <div className="mt-5 pt-4 border-t border-white/10">
        <h4 className="text-white/80 text-sm font-medium mb-2">Available Models</h4>
        <ul className="text-white/50 text-xs space-y-1">
          <li>• glm-4-plus (Chat - Most capable)</li>
          <li>• glm-4-0520 (Chat - Recommended)</li>
          <li>• glm-4-flash (Chat - Fast)</li>
          <li>• glm-4 (Chat - Standard)</li>
        </ul>
      </div>
    </div>
  );
};

export default GLMConfigCard;
