import { type Meta, type Transaction } from '@/api/transaction';
import Field from '@/components/field';
import SectionLabel from '@/components/section-label';
import ToggleButton from '@/components/toggle-button';
import { formatNumber, holderColor } from '@/utils/utils';

interface Props {
    transaction: Transaction;
    meta: Meta;
    onPatch: (id: string, patch: Partial<Transaction>) => void;
}

const DetailPanel = ({ transaction, meta, onPatch }: Props) => {
    const hcolor = holderColor(transaction.cardholder);
    const { owes } = transaction;

    return (
        <div className="p-4 border-b border-(--color-border)">
            <SectionLabel>Transaction</SectionLabel>

            <div className="mt-3 mb-1 text-[10px] text-(--color-muted) tabular-nums">
                {transaction.date}
            </div>
            <div className="text-[14px] font-medium leading-snug mb-3">
                {transaction.description}
            </div>
            <div className="text-[22px] font-semibold tabular-nums mb-4">
                {formatNumber(transaction.amount)}
            </div>

            {/* Cardholder */}
            <Field label="Cardholder">
                <select
                    value={transaction.cardholder}
                    onChange={(event) => {
                        onPatch(String(transaction.id), {
                            cardholder: event.target.value,
                        });
                    }}
                    className="w-full bg-(--color-surface2) border border-(--color-border) font-mono text-[12px] px-2 py-1.5 rounded-sm outline-none cursor-pointer"
                    style={{ color: hcolor }}
                >
                    {meta.cardholders.map((cardholder) => (
                        <option
                            key={cardholder}
                            value={cardholder}
                            style={{
                                background: '#111114',
                                color: holderColor(cardholder),
                            }}
                        >
                            {cardholder}
                        </option>
                    ))}
                </select>
            </Field>

            {/* Category */}
            <Field label="Category">
                <select
                    value={transaction.category}
                    onChange={(event) => {
                        onPatch(String(transaction.id), {
                            category: event.target.value,
                        });
                    }}
                    className="w-full bg-(--color-surface2) border border-(--color-border) text-(--color-text) font-mono text-[12px] px-2 py-1.5 rounded-sm outline-none cursor-pointer"
                >
                    {meta.categories.map((category) => (
                        <option
                            key={category}
                            value={category}
                            style={{ background: '#111114' }}
                        >
                            {category}
                        </option>
                    ))}
                </select>
            </Field>

            {/* Split preview */}
            <Field label="Split">
                <div className="bg-(--color-surface2) border border-(--color-border) rounded-sm px-2 py-1.5 text-[11px] space-y-0.5">
                    {transaction.isShared ? (
                        Object.entries(owes).map(([p, v]) => (
                            <div key={p} className="flex justify-between">
                                <span style={{ color: holderColor(p) }}>
                                    {p}
                                </span>
                                <span className="tabular-nums text-(--color-muted)">
                                    {formatNumber(v)}
                                </span>
                            </div>
                        ))
                    ) : (
                        <span className="text-(--color-muted)">
                            {transaction.cardholder} pays all
                        </span>
                    )}
                </div>
            </Field>

            {/* Share toggle buttons */}
            <div className="flex gap-2 mt-4">
                <ToggleButton
                    active={transaction.isShared}
                    activeColor="var(--color-shared)"
                    onClick={() => {
                        onPatch(String(transaction.id), { isShared: true });
                    }}
                >
                    Shared
                </ToggleButton>
                <ToggleButton
                    active={!transaction.isShared}
                    activeColor="var(--color-muted2)"
                    onClick={() => {
                        onPatch(String(transaction.id), { isShared: false });
                    }}
                >
                    Personal
                </ToggleButton>
            </div>
        </div>
    );
};

export default DetailPanel;
