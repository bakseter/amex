import { type ReactNode } from 'react';

interface Props {
    label: string;
    children: ReactNode;
}

const Field = ({ label, children }: Props) => (
    <div className="mb-3">
        <div className="text-[10px] tracking-[.1em] uppercase text-(--color-muted) mb-1">
            {label}
        </div>
        {children}
    </div>
);

export default Field;
