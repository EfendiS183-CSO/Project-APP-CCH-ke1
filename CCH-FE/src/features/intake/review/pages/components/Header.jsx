
// Assuming you might have user profile data or state
// For now, a placeholder icon will be used.

const Header = ({ title }) => {
    return (
        <header className="bg-white shadow px-4 py-3 flex justify-between items-center">
            <h1 className="text-xl font-semibold text-gray-800">{title}</h1>
            <div className="flex items-center">
                <div className="relative">
                    <input
                        type="text"
                        placeholder="Search..."
                        className="ml-4 px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 w-64"
                    />
                    <svg
                        className="w-4 h-4 absolute right-3 top-1/2 transform -translate-y-1/2 text-gray-500"
                        xmlns="http://www.w3.org/2000/svg"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                    >
                        <circle cx="11" cy="11" r="8"></circle>
                        <line x1="21" y1="21" x2="16.65" y2="16.65"></line>
                    </svg>
                </div>
                <div className="ml-4 w-10 h-10 rounded-full bg-gray-300 flex items-center justify-center cursor-pointer shadow">
                    ðŸ‘¤ {/* Placeholder for user avatar */}
                </div>
            </div>
        </header>
    );
};

export default Header;
