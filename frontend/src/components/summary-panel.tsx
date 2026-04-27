import { type Summary } from '@/api/transaction';
import SectionLabel from '@/components/section-label';
import { formatNumber, holderColor } from '@/utils/utils';

interface Props {
    summary: Summary;
}

const SummaryPanel = ({ summary }: Props) => {
    const { personTotals, grandTotal, persons } = summary;

    return (
        <div className="p-4 border-b border-(--color-border) shrink-0">
            <SectionLabel>Summary</SectionLabel>
            <div className="space-y-2 mt-3">
                {persons.map((person) => {
                    const total = personTotals[person];
                    const pct = grandTotal > 0 ? total / grandTotal : 0;
                    const color = holderColor(person);

                    return (
                        <div key={person}>
                            <div className="flex justify-between items-baseline mb-1">
                                <span
                                    className="text-[12px] font-medium"
                                    style={{ color }}
                                >
                                    {person}
                                </span>
                                <span className="tabular-nums text-[12px]">
                                    {formatNumber(total)}
                                </span>
                            </div>
                            <div className="h-0.5 rounded-full bg-(--color-border)">
                                <div
                                    className="h-full rounded-full transition-all duration-300"
                                    style={{
                                        width: `${String(pct * 100)}%`,
                                        background: color,
                                    }}
                                />
                            </div>
                        </div>
                    );
                })}
                <div className="flex justify-between items-baseline pt-2 border-t border-(--color-border) mt-2">
                    <span className="text-[11px] text-(--color-muted) uppercase tracking-widest">
                        Total
                    </span>
                    <span className="tabular-nums text-[13px] font-medium">
                        {formatNumber(grandTotal)}
                    </span>
                </div>
            </div>
        </div>
    );
};

export default SummaryPanel;
