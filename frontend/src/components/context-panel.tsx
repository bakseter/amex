import { type Transaction } from '@/api/transaction';
import SectionLabel from '@/components/section-label';
import { holderColor } from '@/utils/utils';

interface Props {
    nearby: Transaction[];
    selectedId: string | null;
}

const ContextPanel = ({ nearby, selectedId }: Props) => {
    if (!nearby.length) {
        return null;
    }

    return (
        <div className="p-4">
            <SectionLabel>Nearby</SectionLabel>
            <div className="mt-3 space-y-1">
                {nearby.map((transaction) => {
                    const isSel = String(transaction.id) === selectedId;
                    const hcolor = holderColor(transaction.cardholder);

                    return (
                        <div
                            key={transaction.id}
                            className={`rounded-sm px-2.5 py-2 text-[11px] border-l-2 transition-colors ${
                                isSel ? 'bg-[#1a1a28]' : 'opacity-60'
                            }`}
                            style={{
                                borderLeftColor: isSel
                                    ? hcolor
                                    : 'var(--color-border2)',
                            }}
                        >
                            <div className="flex justify-between gap-2">
                                <span
                                    className="truncate max-w-[150px] text-[11px]"
                                    title={transaction.description}
                                >
                                    {transaction.description}
                                </span>
                                <span className="tabular-nums text-(--color-muted) shrink-0">
                                    {new Intl.NumberFormat('nb-NO', {
                                        minimumFractionDigits: 2,
                                    }).format(transaction.amount)}
                                </span>
                            </div>
                            <div className="text-(--color-muted) text-[10px] mt-0.5">
                                {transaction.date}
                                <span className="mx-1.5 opacity-40">·</span>
                                <span style={{ color: hcolor }}>
                                    {transaction.cardholder}
                                </span>
                                <span className="mx-1.5 opacity-40">·</span>
                                <span
                                    style={{
                                        color: transaction.isShared
                                            ? 'var(--color-shared)'
                                            : 'var(--color-muted)',
                                    }}
                                >
                                    {transaction.isShared
                                        ? 'shared'
                                        : 'personal'}
                                </span>
                            </div>
                        </div>
                    );
                })}
            </div>
        </div>
    );
};

export default ContextPanel;
