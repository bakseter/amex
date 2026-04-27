import type { Meta, Transaction } from '@/api/transaction';
import Pill from '@/components/pill';

interface Props {
    meta: Meta;
    transactions: Transaction[];
}

const Header = ({ meta, transactions }: Props) => {
    const shared = transactions.filter(
        (transaction) => transaction.isShared
    ).length;

    const personal = transactions.length - shared;

    return (
        <header className="flex items-center gap-3 px-4 h-11 border-b border-(--color-border) bg-(--color-surface) shrink-0">
            <span className="font-mono text-xs tracking-[.12em] text-(--color-muted2) uppercase">
                AMEX
            </span>
            <span className="text-(--color-border2)">·</span>
            <span className="font-mono text-xs text-(--color-muted) truncate max-w-48">
                {meta.pdf ? meta.pdf.split('/').pop() : '—'}
            </span>

            <div className="flex items-center gap-2 ml-2">
                <Pill color="(--color-shared)">{shared} shared</Pill>
                <Pill color="(--color-muted2)">{personal} personal</Pill>
            </div>
        </header>
    );
};

export default Header;
