import { useContext } from 'react';
import { AuthContext } from '../../auth/auth-context'; // Assuming AuthContext provides role information
import UserDashboard from './UserDashboard'; // Placeholder for user role dashboard
import AdminDashboard from './AdminDashboard'; // Placeholder for admin role dashboard
import DefaultDashboard from './DefaultDashboard'; // Fallback dashboard

const DashboardPage = () => {
    const { user } = useContext(AuthContext); // Assuming user object contains role information

    // Determine which dashboard to render based on user role
    const renderDashboard = () => {
        if (!user?.role) {
            return <DefaultDashboard />; // Render default if no role is found
        }

        switch (user.role) {
            case 'admin':
                return <AdminDashboard />;
            case 'user':
                return <UserDashboard />;
            default:
                return <DefaultDashboard />; // Fallback for unknown roles
        }
    };

    return (
        <div className="dashboard-container">
            {renderDashboard()}
        </div>
    );
};

export default DashboardPage;
