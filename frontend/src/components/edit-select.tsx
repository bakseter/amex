const EditSelect = ({
    value,
    options,
    onChange,
}: {
    value: string;
    options: string[];
    onChange: (v: string) => void;
}) => (
        <select
            value={value}
            onChange={(event) => { onChange(event.target.value); }}
            onClick={(event) => { event.stopPropagation(); }}
            className="text-xs bg-transparent border border-gray-200 rounded px-1 py-0.5 focus:outline-none focus:border-blue-400 max-w-[140px]"
        >
            {options.map((option) => (
                <option key={option}>{option}</option>
            ))}
        </select>
    );

export default EditSelect;
