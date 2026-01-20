import React, { useEffect, useState } from 'react';
import { X, AlertCircle } from 'lucide-react';

export interface ToastMessage {
  id: string;
  message: string;
  type?: 'error' | 'info' | 'success';
}

interface ToastProps {
  toasts: ToastMessage[];
  onDismiss: (id: string) => void;
}

export const Toast: React.FC<ToastProps> = ({ toasts, onDismiss }) => {
  return (
    <div className="fixed top-4 right-4 z-50 flex flex-col gap-2 max-w-md">
      {toasts.map((toast) => (
        <ToastItem key={toast.id} toast={toast} onDismiss={onDismiss} />
      ))}
    </div>
  );
};

const ToastItem: React.FC<{ toast: ToastMessage; onDismiss: (id: string) => void }> = ({ toast, onDismiss }) => {
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    // Animate in
    requestAnimationFrame(() => setIsVisible(true));
    
    // Auto dismiss after 5 seconds
    const timer = setTimeout(() => {
      setIsVisible(false);
      setTimeout(() => onDismiss(toast.id), 300);
    }, 5000);

    return () => clearTimeout(timer);
  }, [toast.id, onDismiss]);

  const bgColor = toast.type === 'error' 
    ? 'bg-red-900/90 border-red-500/50' 
    : toast.type === 'success'
    ? 'bg-green-900/90 border-green-500/50'
    : 'bg-blue-900/90 border-blue-500/50';

  const iconColor = toast.type === 'error' 
    ? 'text-red-400' 
    : toast.type === 'success'
    ? 'text-green-400'
    : 'text-blue-400';

  return (
    <div
      className={`
        flex items-start gap-3 p-4 rounded-lg border backdrop-blur-sm shadow-lg
        transition-all duration-300 ease-out
        ${bgColor}
        ${isVisible ? 'opacity-100 translate-x-0' : 'opacity-0 translate-x-8'}
      `}
    >
      <AlertCircle className={`w-5 h-5 flex-shrink-0 mt-0.5 ${iconColor}`} />
      <p className="text-sm text-gray-100 flex-1">{toast.message}</p>
      <button
        onClick={() => {
          setIsVisible(false);
          setTimeout(() => onDismiss(toast.id), 300);
        }}
        className="text-gray-400 hover:text-white transition-colors flex-shrink-0"
      >
        <X className="w-4 h-4" />
      </button>
    </div>
  );
};
