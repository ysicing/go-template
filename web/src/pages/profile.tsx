import { useQuery } from "@tanstack/react-query";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../components/ui/card";
import { fetchCurrentUser } from "../lib/api";
import { ChangePasswordCard } from "../subsystems/auth/components/change-password-card";

function ProfileSummaryCard() {
  const query = useQuery({
    queryKey: ["auth-me"],
    queryFn: fetchCurrentUser
  });

  return (
    <Card>
      <CardHeader>
        <CardTitle>Profile</CardTitle>
        <CardDescription>当前登录用户信息</CardDescription>
      </CardHeader>
      <CardContent className="space-y-2 text-sm">
        {query.isLoading ? <div className="text-muted-foreground">Loading profile...</div> : null}
        <div>Username: {query.data?.username ?? "-"}</div>
        <div>Email: {query.data?.email ?? "-"}</div>
        <div>Role: {query.data?.role ?? "-"}</div>
      </CardContent>
    </Card>
  );
}

export function ProfilePage() {
  return (
    <div className="space-y-6">
      <ProfileSummaryCard />
      <ChangePasswordCard />
    </div>
  );
}
