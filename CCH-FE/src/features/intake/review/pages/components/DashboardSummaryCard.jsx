
const DashboardSummaryCard = ({ title, value, icon, color }) => {
    return (
        <div className="bg-white p-6 rounded-lg shadow-md flex justify-between items-center transition duration-300 ease-in-out hover:shadow-lg transform hover:-translate-y-1">
            <div className="flex flex-col">
                <h3 className="text-lg font-medium text-gray-600 mb-1">{title}</h3>
                <p className={`text-3xl font-bold ${color}`}>{value}</p>
            </div>
            <div className={`text-4xl opacity-50 ${color === 'text-blue-600' ? 'text-blue-400' : color === 'text-orange-600' ? 'text-orange-400' : color === 'text-green-600' ? 'text-green-400' : 'text-purple-400'}`}>
                {icon}
            </div>
        </div>
    );
};

export default DashboardSummaryCard;
