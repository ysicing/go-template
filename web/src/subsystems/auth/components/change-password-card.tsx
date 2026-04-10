import { FormEvent, useState } from "react";

import { Button } from "../../../components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../../../components/ui/card";
import { Input } from "../../../components/ui/input";
import { Label } from "../../../components/ui/label";
import { changePassword } from "../../../lib/api";

function getErrorMessage(error: unknown) {
  if (
    typeof error === "object" &&
    error !== null &&
    "response" in error &&
    typeof error.response === "object" &&
    error.response !== null &&
    "data" in error.response &&
    typeof error.response.data === "object" &&
    error.response.data !== null &&
    "message" in error.response.data &&
    typeof error.response.data.message === "string"
  ) {
    return error.response.data.message;
  }

  if (error instanceof Error && error.message) {
    return error.message;
  }

  return "密码修改失败";
}

export function ChangePasswordCard() {
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmNewPassword, setConfirmNewPassword] = useState("");
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  function resetMessages() {
    if (error) {
      setError("");
    }

    if (success) {
      setSuccess("");
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setSuccess("");

    const trimmedOldPassword = oldPassword.trim();
    const trimmedNewPassword = newPassword.trim();
    const trimmedConfirmNewPassword = confirmNewPassword.trim();

    if (!trimmedOldPassword || !trimmedNewPassword || !trimmedConfirmNewPassword) {
      setError("请完整填写所有密码字段");
      return;
    }

    if (trimmedNewPassword !== trimmedConfirmNewPassword) {
      setError("两次输入的新密码不一致");
      return;
    }

    setIsSubmitting(true);

    try {
      const result = await changePassword({
        old_password: trimmedOldPassword,
        new_password: trimmedNewPassword,
        confirm_new_password: trimmedConfirmNewPassword
      });

      if (!result.changed) {
        setError("密码修改失败");
        return;
      }

      setOldPassword("");
      setNewPassword("");
      setConfirmNewPassword("");
      setSuccess("密码修改成功");
    } catch (submitError) {
      setError(getErrorMessage(submitError));
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>修改密码</CardTitle>
        <CardDescription>更新当前账号的登录密码</CardDescription>
      </CardHeader>
      <CardContent>
        <form className="space-y-4" onSubmit={handleSubmit}>
          <div className="space-y-2">
            <Label htmlFor="old-password">旧密码</Label>
            <Input
              id="old-password"
              onChange={(event) => {
                resetMessages();
                setOldPassword(event.target.value);
              }}
              type="password"
              value={oldPassword}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="new-password">新密码</Label>
            <Input
              id="new-password"
              onChange={(event) => {
                resetMessages();
                setNewPassword(event.target.value);
              }}
              type="password"
              value={newPassword}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="confirm-new-password">确认新密码</Label>
            <Input
              id="confirm-new-password"
              onChange={(event) => {
                resetMessages();
                setConfirmNewPassword(event.target.value);
              }}
              type="password"
              value={confirmNewPassword}
            />
          </div>
          {error ? <p className="text-sm text-red-500">{error}</p> : null}
          {success ? <p className="text-sm text-green-600">{success}</p> : null}
          <Button className="w-full" disabled={isSubmitting} type="submit">
            {isSubmitting ? "提交中..." : "提交"}
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
