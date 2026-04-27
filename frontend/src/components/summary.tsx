import { useGetSummaryQuery } from '@/api/transaction';

const Summary = ({ invoiceId }: { invoiceId: number }) => {
    const { data } = useGetSummaryQuery(invoiceId);

    if (!data) {
        return null;
    }

    return (
        <div className="text-sm space-y-3">
            <div className="font-semibold text-gray-700">Summary</div>
            <div className="space-y-1">
                {data.persons.map((person) => (
                    <div key={person} className="flex justify-between text-xs">
                        <span className="text-gray-500">{person} owes</span>
                        <span className="font-mono font-medium">
                            {data.personTotals[person].toFixed(2)} NOK
                        </span>
                    </div>
                ))}
                <div className="flex justify-between text-xs border-t pt-1 mt-1">
                    <span className="text-gray-500">Total</span>
                    <span className="font-mono font-semibold">
                        {data.grandTotal.toFixed(2)} NOK
                    </span>
                </div>
            </div>
            <div className="space-y-0.5 max-h-64 overflow-y-auto">
                {data.byCategory.map(([cat, vals]) => (
                    <div
                        key={cat}
                        className="flex justify-between text-xs text-gray-500"
                    >
                        <span className="truncate mr-2">{cat}</span>
                        <span className="font-mono shrink-0">
                            {vals.total.toFixed(2)}
                        </span>
                    </div>
                ))}
            </div>
        </div>
    );
};

export default Summary;
