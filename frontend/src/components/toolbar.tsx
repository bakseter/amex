import type { ReactNode } from 'react';

import type { Meta } from '@/api/transaction';

interface Props {
    filters: {
        query: string;
        holder: string;
        category: string;
        uncategorized: boolean;
    };
    setFilters: (fn: (f) => any) => void;
    meta: Meta;
    visibleCount: number;
}

interface SelectProps {
    value: string;
    onChange: (v: string) => void;
    children: ReactNode;
}

const Select = ({ value, onChange, children }: SelectProps) => (
    <select
        value={value}
        onChange={(event) => {
            onChange(event.target.value);
        }}
        className="bg-(--color-surface) border border-(--color-border) text-(--color-muted2) font-mono text-[11px] px-2 py-1.5 rounded-sm outline-none cursor-pointer hover:text-(--color-text)"
    >
        {children}
    </select>
);

const Toolbar = ({ filters, setFilters, meta, visibleCount }: Props) => {
    const set = (key, val) => {
        setFilters((filter) => ({ ...filter, [key]: val }));
    };

    return (
        <div className="flex items-center gap-2 px-4 py-2 border-b border-(--color-border) bg-(--color-bg) sticky top-0 z-10 shrink-0">
            <input
                type="search"
                placeholder="Search…"
                value={filters.query}
                onChange={(event) => {
                    set('q', event.target.value);
                }}
                className="bg-(--color-surface) border border-(--color-border) text-(--color-text) placeholder-text-(--color-muted) font-mono text-xs px-3 py-1.5 rounded-sm outline-none w-48 focus:border-(--color-border2)"
            />

            <Select
                value={filters.holder}
                onChange={(value) => {
                    set('holder', value);
                }}
            >
                <option value="">All cardholders</option>
                {meta.cardholders.map((cardholder) => (
                    <option key={cardholder} value={cardholder}>
                        {cardholder}
                    </option>
                ))}
            </Select>

            <Select
                value={filters.category}
                onChange={(value) => {
                    set('category', value);
                }}
            >
                <option value="">All categories</option>
                {meta.categories.map((category) => (
                    <option key={category} value={category}>
                        {category}
                    </option>
                ))}
            </Select>

            <label className="flex items-center gap-1.5 text-[11px] text-(--color-muted) cursor-pointer select-none">
                <input
                    type="checkbox"
                    checked={filters.uncategorized}
                    onChange={(event) => {
                        set('uncategorized', event.target.checked);
                    }}
                    className="accent-(--color-shared)"
                />
                Uncategorized
            </label>

            <div className="flex-1" />
            <span className="text-[11px] text-(--color-muted) tabular-nums">
                {visibleCount} rows
            </span>
        </div>
    );
};

export default Toolbar;
