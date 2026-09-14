// components/summary.tsx
import { useGetMetaQuery, useGetSummaryQuery } from '@/api/transaction';
import { formatNumber, personColor } from '@/utils/utils';

const Summary = ({ invoiceId }: { invoiceId: number }) => {
    const { data } = useGetSummaryQuery(invoiceId);
    const { data: meta } = useGetMetaQuery();

    if (!data) {
        return null;
    }

    const largest = data.byCategory[0]?.[1].total ?? 0;

    return (
        <div className="flex flex-col gap-6">
            <div className="flex flex-col gap-4">
                {data.persons.map((person) => (
                    <div key={person} className="flex flex-col gap-0.5">
                        <div className="text-xs text-muted2">{person} owes</div>
                        <div
                            className="text-2xl font-semibold tabular-nums"
                            style={{ color: personColor(person, data.persons) }}
                        >
                            {formatNumber(data.personTotals[person] ?? 0)}
                        </div>
                    </div>
                ))}
            </div>

            {/* Proportional bar: the split made visible, no legend needed. */}
            <div className="flex h-1.5 overflow-hidden rounded-full bg-surface2">
                {data.persons.map((person) => (
                    <div
                        key={person}
                        style={{
                            width: `${String(
                                data.grandTotal > 0
                                    ? ((data.personTotals[person] ?? 0) /
                                          data.grandTotal) *
                                          100
                                    : 0
                            )}%`,
                            background: personColor(person, data.persons),
                        }}
                    />
                ))}
            </div>

            <div className="flex items-baseline justify-between border-t border-border pt-3">
                <span className="text-muted2">Statement total</span>
                <span className="font-medium tabular-nums">
                    {formatNumber(data.grandTotal)}
                </span>
            </div>

            <div className="flex flex-col gap-2.5">
                <div className="text-xs text-muted2">Where it went</div>
                {data.byCategory.map(([category, values]) => (
                    <div key={category} className="flex flex-col gap-1">
                        <div className="flex items-baseline justify-between gap-2 text-xs">
                            <span className="truncate">{category}</span>
                            <span className="shrink-0 tabular-nums text-muted2">
                                {formatNumber(values.total)}
                            </span>
                        </div>
                        <div className="h-1 rounded-full bg-surface2">
                            <div
                                className="h-1 rounded-full bg-border2"
                                style={{
                                    width: `${String(
                                        largest > 0
                                            ? (values.total / largest) * 100
                                            : 0
                                    )}%`,
                                }}
                            />
                        </div>
                    </div>
                ))}
            </div>

            {meta && (
                <div className="text-xs text-muted">
                    Shared costs split{' '}
                    {data.persons
                        .map(
                            (person) =>
                                `${((meta.splitRatio[person] ?? 0) * 100).toFixed(0)}%`
                        )
                        .join(' / ')}
                </div>
            )}
        </div>
    );
};

export default Summary;
