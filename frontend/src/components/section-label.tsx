import { type ReactNode } from 'react';

interface Props {
    children: ReactNode;
}

const SectionLabel = ({ children }: Props) => (
    <span className="text-[10px] tracking-[.14em] uppercase text-(--color-muted)">
        {children}
    </span>
);

export default SectionLabel;
