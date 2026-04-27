import { type ReactNode } from 'react';

interface Props {
    active: boolean;
    activeColor: string;
    onClick: () => void;
    children: ReactNode;
}

const ToggleButton = ({ active, activeColor, onClick, children }: Props) => (
    <button
        onClick={onClick}
        className="flex-1 py-1.5 text-[11px] border rounded-sm transition-all tracking-[.04em]"
        style={
            active
                ? {
                      borderColor: activeColor,
                      color: activeColor,
                      background: `color-mix(in srgb, ${activeColor} 10%, transparent)`,
                  }
                : {
                      borderColor: 'var(--color-border)',
                      color: 'var(--color-muted)',
                  }
        }
    >
        {children}
    </button>
);

export default ToggleButton;
