import { Link } from 'react-router-dom';

const Sidebar = () => {
    // Navigation links - you can expand this array as needed
    const navLinks = [
        { to: '/dashboard', text: 'Dashboard', icon: 'fas fa-tachometer-alt' },
        { to: '/all-complaints', text: 'All Complaints', icon: 'fas fa-exclamation-circle' },
        { to: '/analytics', text: 'Analytics', icon: 'fas fa-chart-bar' },
        { to: '/reports', text: 'Reports', icon: 'fas fa-table' },
        { to: '/settings', text: 'Settings', icon: 'fas fa-cog' },
    ];

    return (
        <div className="w-64 bg-gray-800 text-white flex flex-col p-4">
            <div className="text-2xl font-bold mb-8 text-center">RESOLVEHUB</div>
            <nav className="flex-1 space-y-2">
                {navLinks.map((link, index) => (
                    <Link
                        key={index}
                        to={link.to}
                        className="block py-2.5 px-4 rounded transition duration-200 hover:bg-gray-700 hover:text-white flex items-center text-gray-300"
                    >
                        <i className={`${link.icon} mr-3`}></i>
                        <span>{link.text}</span>
                    </Link>
                ))}
            </nav>
            {/* Optional: Add User info or other elements at the bottom */}
            <div className="mt-auto p-4 text-center text-gray-500 text-sm">
                App Version 1.0.0
            </div>
        </div>
    );
};

export default Sidebar;
