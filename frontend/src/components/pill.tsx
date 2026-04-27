import { type ReactNode } from 'react';

interface Props {
    color: string;
    children: ReactNode;
}

const Pill = ({ color, children }: Props) => (
    <span
        className="text-[10px] tracking-[.06em] px-2 py-0.5 rounded-sm border"
        style={{
            color: `var(${color})`,
            borderColor: `color-mix(in srgb, var(${color}) 25%, transparent)`,
            background: `color-mix(in srgb, var(${color}) 8%, transparent)`,
        }}
    >
        {children}
    </span>
);

export default Pill;
